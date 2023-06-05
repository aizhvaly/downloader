package downloader

import (
	"fmt"
	"net/http"
	"strings"
)

// if file size less than chunk size or server does not support range - just download it direct from one source
// try another source if failed
func (d *downloader) simpleDownload(f *file) error {
	errSlice := make([]string, 0, len(f.urls))
	for _, url := range f.urls {
		resp, err := http.DefaultClient.Get(url)
		if err != nil {
			errSlice = append(errSlice, fmt.Sprintf("source %s; err: %s", url, err.Error()))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			errSlice = append(errSlice, fmt.Sprintf("source: %s; err: bad status code: %d", url, resp.StatusCode))
			continue
		}

		if err := f.write(resp.Body, d.withSync); err != nil {
			errSlice = append(errSlice, fmt.Sprintf("failed to copy data from resp.Body; err: %s", err.Error()))

			if _, err := f.fd.Seek(0, 0); err != nil {
				return fmt.Errorf("failed to erase corrupt data from file: %v", err)
			}

			continue
		}

		return nil
	}

	return fmt.Errorf("failed to download file %s: %s", f.name(), strings.Join(errSlice, ","))
}
