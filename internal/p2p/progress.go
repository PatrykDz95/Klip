package p2p

import (
	"io"
	"time"
)

type progressReader struct {
	reader      io.Reader
	total       int64
	transferred int64
	fileName    string
	progressCh  chan FileProgress
	lastReport  time.Time
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.transferred += int64(n)

	if time.Since(pr.lastReport) > 100*time.Millisecond {
		select {
		case pr.progressCh <- FileProgress{
			FileName:    pr.fileName,
			Transferred: pr.transferred,
			Total:       pr.total,
		}:
		default: // don't block if the channel is full
		}
		pr.lastReport = time.Now()
	}

	return n, err
}

// SendDone sends a final Done=true progress event after transfer completes.
func (pr *progressReader) SendDone() {
	pr.progressCh <- FileProgress{
		FileName:    pr.fileName,
		Transferred: pr.total,
		Total:       pr.total,
		Done:        true,
	}
}

func copyBufferN(dst io.Writer, src io.Reader, n int64, buf []byte) (int64, error) {
	return io.CopyBuffer(dst, io.LimitReader(src, n), buf)
}
