package p2p

import (
	"net"
	"time"
)

type deadlineReader struct {
	conn    net.Conn
	timeout time.Duration
}

// Read sets a read deadline before reading from the connection.
// If the read operation takes longer than the specified timeout, it will return an error.
func (dr *deadlineReader) Read(p []byte) (int, error) {
	if err := dr.conn.SetReadDeadline(time.Now().Add(dr.timeout)); err != nil {
		return 0, err
	}
	return dr.conn.Read(p)
}

type deadlineWriter struct {
	conn    net.Conn
	timeout time.Duration
}

// Write sets a write deadline before writing to the connection.
// If the write operation takes longer than the specified timeout, it will return an error.
func (dw *deadlineWriter) Write(p []byte) (int, error) {
	if err := dw.conn.SetWriteDeadline(time.Now().Add(dw.timeout)); err != nil {
		return 0, err
	}
	return dw.conn.Write(p)
}
