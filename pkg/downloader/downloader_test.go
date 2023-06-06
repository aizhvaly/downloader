package downloader

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDownloader(t *testing.T) {
	testCases := []struct {
		name               string
		correct            bool
		acceptRangeSrv1    bool
		acceptRangeSrv2    bool
		zeroContentLenSrv1 bool
		zeroContentLenSrv2 bool
		withSync           bool
		workersPerFile     int
		errN               int
		sources            map[string][]string
		chunkSize          int64
	}{
		{
			name:               "correct_2_sources",
			correct:            true,
			acceptRangeSrv1:    true,
			acceptRangeSrv2:    true,
			zeroContentLenSrv1: false,
			withSync:           false,
			workersPerFile:     5,
			errN:               0,
			sources:            map[string][]string{"test-file.txt": []string{"http://127.0.0.1:8080/test-file.txt", "http://127.0.0.1:8081/test-file.txt"}},
			chunkSize:          1000 * 1000,
		},
		{
			name:               "correct_small_chunk_size",
			correct:            true,
			acceptRangeSrv1:    true,
			acceptRangeSrv2:    true,
			zeroContentLenSrv1: false,
			withSync:           false,
			workersPerFile:     5,
			errN:               0,
			sources:            map[string][]string{"test-file.txt": []string{"http://127.0.0.1:8080/test-file.txt", "http://127.0.0.1:8081/test-file.txt"}},
			chunkSize:          512 * 1000,
		},
		{
			name:               "correct_big_chunk_size",
			correct:            true,
			acceptRangeSrv1:    true,
			acceptRangeSrv2:    true,
			zeroContentLenSrv1: false,
			withSync:           false,
			workersPerFile:     5,
			errN:               0,
			sources:            map[string][]string{"test-file.txt": []string{"http://127.0.0.1:8080/test-file.txt", "http://127.0.0.1:8081/test-file.txt"}},
			chunkSize:          25 * 1000 * 1000,
		},
		{
			name:               "correct_1_source",
			correct:            true,
			acceptRangeSrv1:    true,
			acceptRangeSrv2:    false,
			zeroContentLenSrv1: false,
			withSync:           false,
			workersPerFile:     5,
			errN:               0,
			sources:            map[string][]string{"test-file.txt": []string{"http://127.0.0.1:8080/test-file.txt", "http://127.0.0.1:8081/test-file.txt"}},
			chunkSize:          1000 * 1000,
		},
		{
			name:               "incorrect_zero_content_len",
			correct:            false,
			zeroContentLenSrv1: true,
			zeroContentLenSrv2: true,
			withSync:           false,
			workersPerFile:     5,
			errN:               0,
			sources:            map[string][]string{"test-file.txt": []string{"http://127.0.0.1:8080/test-file.txt", "http://127.0.0.1:8081/test-file.txt"}},
			chunkSize:          1000 * 1000,
		},
		{
			name:               "incorrect_no_sources",
			correct:            false,
			zeroContentLenSrv1: true,
			zeroContentLenSrv2: true,
			withSync:           false,
			workersPerFile:     5,
			errN:               0,
			sources:            map[string][]string{},
			chunkSize:          1000 * 1000,
		},
	}

	data1 := make([]byte, 60*1000*1000)
	for i := range data1 {
		data1[i] = byte('a' + i%26)
	}

	data2 := make([]byte, 60*1000*1000)
	copy(data2, data1)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mux1 := http.NewServeMux()
			mux2 := http.NewServeMux()
			mux1.HandleFunc("/test-file.txt", handle(data1, tc.acceptRangeSrv1, tc.zeroContentLenSrv1))
			mux2.HandleFunc("/test-file.txt", handle(data2, tc.acceptRangeSrv2, tc.zeroContentLenSrv2))

			srv1 := &http.Server{
				Addr:    ":8080",
				Handler: mux1,
			}

			srv2 := &http.Server{
				Addr:    ":8081",
				Handler: mux2,
			}

			go func() { srv1.ListenAndServe() }()
			go func() { srv2.ListenAndServe() }()
			defer srv1.Close()
			defer srv2.Close()

			d, err := New(&DownloaderConfig{
				WithSync:        tc.withSync,
				WorkersPerFile:  tc.workersPerFile,
				Retries:         3,
				MaxFailedChunks: 3,
				ChunkSize:       tc.chunkSize,
				Sources:         tc.sources,
			})

			if !tc.correct {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)

			err = d.download(context.Background(), d.files[0], d.workersPerFile)
			assert.Nil(t, err)

			fd, err := os.Open("test-file.txt")
			assert.Nil(t, err)
			defer fd.Close()

			compareData, err := io.ReadAll(fd)
			assert.Nil(t, err)
			n := bytes.Compare(data1, compareData)
			assert.Equal(t, 0, n)
			os.Remove("test-file.txt")
		})
	}
}

func handle(data []byte, acceptRange, zeroContent bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			if zeroContent {
				w.Header().Set("Content-Length", "0")
			} else {
				w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			}

			if acceptRange {
				w.Header().Set("Accept-Ranges", "bytes")
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			http.Error(w, "empty range", http.StatusNotAcceptable)
			return
		}
		ranges := strings.Split(rangeHeader, "=")
		byteRange := strings.Split(ranges[1], "-")

		start, err := strconv.ParseInt(byteRange[0], 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		end, err := strconv.ParseInt(byteRange[1], 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[start : end+1])
	}
}
