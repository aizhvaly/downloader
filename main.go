package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"runtime"
	rdebug "runtime/debug"
	"strings"

	"github.com/aizhvaly/downloader/pkg/downloader"
	"github.com/spf13/pflag"
)

var (
	help         bool
	withSync     bool
	retries      int
	failedChunks int
	chunkSize    int64
	sources      []string
)

const helpMessage = `
Download files from multiple urls.
You could download multiple files from multiple urls. Also you could download one file from multiple urls.

Usage:
	downloader [flags]

Flags:
	-h, --help		Help message
	-w, --with-sync		Download with sync means using file sync after successful download. Default: false
	-c, --chunk-size	Chunk size in bytes. Default: 5MB. Min 512KB, Max 30MB.
	-f, --failed-chunks	Max	count of failed chunks to retry. Default: 10
	-r, --retries		Max retries for every failed download file chunk. Default: 3
	-s, --sources		Source file urls. Shoud be passed with "," as separator without space
	
Example:
	downloader -s https://example.com/file1,https://example.com/file2 - for downloading two files from example.com
	downloader -s https://example1.com/file1,https://example2.com/file1 - for downloading one file from different sources
`

var (
	minChunkSize int64 = 512 * 1000
	maxChunkSize int64 = 30 * 1000 * 1000
)

func main() {
	pflag.BoolVarP(&help, "help", "h", false, "Help message")
	pflag.BoolVarP(&withSync, "with-sync", "w", false, "Download with sync every 5% of progress. Default: false")
	pflag.Int64VarP(&chunkSize, "chunk-size", "c", 5*1000*1000, "Chunk size in bytes. Default: 5MB. Min 512KB, Max 30MB.")
	pflag.IntVarP(&retries, "retries", "r", 3, "Max retries for failed file chunks. Default: 3")
	pflag.IntVarP(&failedChunks, "failed-chunks", "f", 10, "Max count of failed chunks to retry. Default: 10")
	pflag.StringSliceVarP(&sources, "sources", "s", []string{}, "Source file urls. Shoud be passed with `,` as separator without spaces.")
	pflag.Parse()
	n := runtime.NumCPU()

	if help {
		fmt.Printf("%s", helpMessage)
		os.Exit(0)
	}

	if len(sources) == 0 {
		log.Fatal("Please provide source urls")
	}

	sourcesMap := make(map[string][]string)

	for _, source := range sources {
		u, err := url.ParseRequestURI(source)
		if err != nil {
			fmt.Printf("Invalid source url: %s\n", source)
			continue
		}

		if u.Path == "" {
			fmt.Printf("Invalid source url: %s\n", source)
			continue
		}

		v := strings.LastIndex(source, "/")
		if v == -1 || v == len(source)-1 {
			fmt.Printf("Invalid source url: %s\n", source)
			continue
		}

		fileName := source[v+1:]
		urls, ok := sourcesMap[fileName]
		if !ok {
			urls = []string{source}
			sourcesMap[fileName] = urls
			continue
		}

		urls = append(urls, source)
		sourcesMap[fileName] = urls
	}

	if len(sourcesMap) == 0 {
		log.Fatal("No valid source urls found, sources: ", sources)
	}

	if chunkSize < minChunkSize {
		chunkSize = minChunkSize
	}

	if chunkSize > maxChunkSize {
		chunkSize = maxChunkSize
	}
	d, err := downloader.New(&downloader.DownloaderConfig{
		WithSync:        withSync,
		WorkersPerFile:  n,
		Retries:         retries,
		MaxFailedChunks: failedChunks,
		ChunkSize:       chunkSize,
		Sources:         sourcesMap,
	})
	if err != nil {
		log.Fatal(err)
	}

	rdebug.SetGCPercent(100 * n)
	if err := d.Download(); err != nil {
		log.Fatal(err)
	}
}
