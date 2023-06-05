package downloader

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"
)

type file struct {
	fd           *os.File
	urls         []string
	size         int64
	progress     int64
	acceptRanges bool
}

func (f *file) close() error { return f.fd.Close() }

func (f *file) name() string { return f.fd.Name() }

func (f *file) getProgress() int64 { return atomic.LoadInt64(&f.progress) }

func (f *file) getURL(n int) string { return f.urls[(n+1)%len(f.urls)] }

func (f *file) write(src io.Reader, sync bool) error {
	if _, err := io.Copy(f.fd, src); err != nil {
		return err
	}

	if sync {
		if err := f.fd.Sync(); err != nil {
			log.Printf("failed to sync %s: %v", f.name(), err)
			return nil
		}
	}

	atomic.StoreInt64(&f.progress, f.size)
	return nil
}

type chunk struct {
	start int64
	stop  int64
	data  *bytes.Buffer
}

func (f *file) writeChunks(ctx context.Context, sourceCh <-chan chunk, pool *sync.Pool, sync bool) error {
l:
	for {
		select {
		case <-ctx.Done():
			break l
		case c, ok := <-sourceCh:
			if !ok {
				break l
			}

			_, err := f.fd.WriteAt(c.data.Bytes(), c.start)
			if err != nil {
				return err
			}

			atomic.AddInt64(&f.progress, c.stop-c.start)
			c.data.Reset()
			pool.Put(c.data)
		}
	}

	if sync {
		if err := f.fd.Sync(); err != nil {
			log.Printf("failed to sync %s: %v", f.name(), err)
			return nil
		}
	}

	return nil
}
