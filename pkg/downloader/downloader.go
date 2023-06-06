package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	mpb "github.com/vbauerster/mpb/v8"
)

type downloader struct {
	withSync        bool
	workersPerFile  int
	retries         int
	maxFailedChunks int
	totalSize       int64
	chunkSize       int64
	files           []*file
}

type DownloaderConfig struct {
	WithSync        bool
	WorkersPerFile  int
	Retries         int
	MaxFailedChunks int
	ChunkSize       int64
	Sources         map[string][]string
}

func New(cfg *DownloaderConfig) (*downloader, error) {
	if len(cfg.Sources) == 0 {
		return nil, errors.New("no sources provided")
	}

	files := make([]*file, 0, len(cfg.Sources))
	var totalSize int64
l:
	for name, urls := range cfg.Sources {

		var acceptRange bool
		var size int64

		if len(urls) == 1 {
			var err error
			size, acceptRange, err = getMetadata(urls[0])
			if err != nil {
				log.Printf("Failed to check the only one source %s: %v", urls[0], err)
				continue
			}
		}

		if len(urls) > 1 {
			sizes := make([]int64, 0, len(urls))
			acceptRanges := make([]bool, 0, len(urls))
			validURLs := make([]string, 0, len(urls))

		s:
			for _, url := range urls {
				var err error
				size, acceptRange, err = getMetadata(url)
				if err != nil {
					log.Printf("Failed to check source %s: %v", url, err)
					continue
				}

				if len(sizes) == 0 {
					sizes = append(sizes, size)
					acceptRanges = append(acceptRanges, acceptRange)
					validURLs = append(validURLs, url)
					continue
				}

				for _, s := range sizes {
					if s != size {
						log.Printf("Found different sizes to check source %s: skip processing this file", url)
						continue l
					}
				}

				for i, ar := range acceptRanges {
					if (ar && !acceptRange) || (!ar && !acceptRange) {
						continue s

					}

					if !ar && acceptRange {
						sizes[i] = size
						acceptRanges[i] = acceptRange
						validURLs[i] = url
						continue s
					}
				}

				// we have the same size, both sources accept range
				sizes = append(sizes, size)
				acceptRanges = append(acceptRanges, acceptRange)
				validURLs = append(validURLs, url)
			}

			urls = validURLs
		}

		if len(urls) == 0 {
			log.Printf("No valid source urls found for file %s, skip\n", name)
			continue
		}

		fd, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Printf("failed to open file %s: %v\n", name, err)
			continue
		}

		files = append(files, &file{
			fd:           fd,
			urls:         urls,
			size:         size,
			acceptRanges: acceptRange,
		})

		totalSize += size
	}

	if len(files) == 0 {
		return nil, errors.New("no valid sources provided")
	}

	return &downloader{
		withSync:        cfg.WithSync,
		retries:         cfg.Retries,
		maxFailedChunks: cfg.MaxFailedChunks,
		workersPerFile:  cfg.WorkersPerFile,
		chunkSize:       cfg.ChunkSize,
		totalSize:       totalSize,
		files:           files,
	}, nil
}

func (d *downloader) Download() error {
	ctx, cancel := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}
	mb := mpb.New()

	for _, fl := range d.files {
		log.Printf("Starting download file %s; sources:\n %v", fl.name(), fl.urls)
	}

	for _, fl := range d.files {
		wg.Add(1)
		f := fl
		defer f.close()

		b := newBar(mb, f.size, f.name())
		runBar(ctx, b, f.getProgress)

		go func(f *file) {
			defer wg.Done()

			if f.size < d.chunkSize || !f.acceptRanges {
				err := d.simpleDownload(f)
				if err != nil {
					log.Printf("Error occured while download file %s: %v\n", f.name(), err)
				}

				return
			}

			if err := d.download(ctx, f, d.workersPerFile); err != nil {
				log.Printf("Error occured while download file %s: %v\n", f.name(), err)
			}

		}(f)

	}

	wg.Wait()
	cancel()
	return nil
}

func (d *downloader) download(ctx context.Context, f *file, n int) error {
	if n <= 0 {
		return fmt.Errorf("invalid number of workers: %d", n)
	}

	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	retrier := newR(d.maxFailedChunks)
	pool := &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, d.chunkSize))
		},
	}

	errList := make([]string, 0)
	errCh := make(chan error)
	go func() {
		for e := range errCh {
			errList = append(errList, e.Error())
		}
	}()

	nChunks := int(math.Ceil(float64(f.size) / float64(d.chunkSize)))

	processCh := make(chan chunk, nChunks/d.workersPerFile+1)
	writeCh := make(chan chunk, nChunks/d.workersPerFile+1)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := f.writeChunks(newCtx, writeCh, pool, d.withSync); err != nil {
			errCh <- fmt.Errorf("failed to write chunks: %w", err)
			cancel()
		}
	}()

	// intentional leak
	// it will not close properly if all workers will fail
	go func() {
		defer close(processCh)

		var i int64
		for i = 0 * d.chunkSize; i < f.size; i += d.chunkSize {
			processCh <- chunk{start: int64(i), stop: min(f.size, int64(i)+d.chunkSize) - 1}
		}
	}()

	workers_wg := &sync.WaitGroup{}
	for i := 0; i < n; i++ {
		workers_wg.Add(1)
		go func(i int) {
			defer workers_wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case ch, ok := <-processCh:
					if !ok {
						return
					}

					if err := processChunk(newCtx, f.getURL(i), ch, pool, writeCh); err != nil {
						if err2 := retrier.add(ch); err2 != nil {
							errCh <- fmt.Errorf("failed to add chunk to retry: %v/%v", err, err2)
							cancel()
							return
						}
					}
				}
			}

		}(i)
	}
	workers_wg.Wait()

	faieldChunks := retrier.getAll()
l:
	for i, ch := range faieldChunks {
		for j := 0; j < d.retries; j++ {
			k := i + j
			if err := processChunk(newCtx, f.getURL(k), ch, pool, writeCh); err != nil {
				if j == d.retries-1 {
					errCh <- fmt.Errorf("failed to process chunk %d-%d : %v", ch.start, ch.stop, err)
					cancel()
					break l
				}

				continue
			}

			continue l
		}
	}

	close(writeCh)
	wg.Wait()
	close(errCh)

	if len(errList) > 0 {
		return fmt.Errorf("%s", strings.Join(errList, " |--> "))
	}

	return nil
}

func processChunk(ctx context.Context, url string, ch chunk, pool *sync.Pool, writeCh chan<- chunk) error {
	// in case of really low bandwidth
	tctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(tctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Range", "bytes="+strconv.Itoa(int(ch.start))+"-"+strconv.Itoa(int(ch.stop)))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get chunk: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("response status code is not appropriate: %d", res.StatusCode)
	}

	b, ok := pool.Get().(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("failed to get buffer from pool")

	}

	if _, err := io.Copy(b, res.Body); err != nil {
		return fmt.Errorf("failed to write data to buffer: %v", err)
	}

	ch.data = b
	writeCh <- ch

	return nil

}

func getMetadata(url string) (int64, bool, error) {
	resp, err := http.Head(url)
	if err != nil {
		return 0, false, fmt.Errorf("error occured while check source url %s: %v", url, err)
	}

	if resp.StatusCode == http.StatusMethodNotAllowed {
		resp, err = http.Get(url)
		if err != nil {
			return 0, false, fmt.Errorf("error occured while check source url %s: %v", url, err)
		}
	}
	defer resp.Body.Close()

	s, ar, err := parseHeaders(resp)
	if err != nil {
		return 0, false, fmt.Errorf("error occured while check source url %s: %v", url, err)

	}

	return s, ar, nil
}

func parseHeaders(resp *http.Response) (int64, bool, error) {
	contentLenght, ok := resp.Header["Content-Length"]
	if !ok {
		return 0, false, errors.New("Content-Length is not provided")
	}

	if len(contentLenght) == 0 || contentLenght[0] == "" {
		return 0, false, errors.New("Content-Length is not provided")
	}

	lenght, err := strconv.ParseInt(contentLenght[0], 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("failed to parse Content-Length: %v", err)
	}

	if lenght == 0 {
		return 0, false, errors.New("Content-Length is 0")
	}

	return lenght, resp.Header.Get("Accept-Ranges") == "bytes", nil
}

func min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}
