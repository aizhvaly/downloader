package downloader

import (
	"context"
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

func newBar(mb *mpb.Progress, fileSize int64, fileName string) *mpb.Bar {
	return mb.AddBar(fileSize,
		mpb.PrependDecorators(
			decor.Name("Downloading "),
			decor.Name(fileName),
			decor.Counters(decor.SizeB1000(0), " % .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.OnComplete(decor.Percentage(decor.WC{W: 5}), ""),
		),
	)
}

func runBar(ctx context.Context, b *mpb.Bar, progressFunc func() int64) {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		// defer wg.Done()
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !b.Completed() {
					b.SetCurrent(progressFunc())
					continue
				}

				return
			case <-ctx.Done():
				return
			}
		}
	}()
}
