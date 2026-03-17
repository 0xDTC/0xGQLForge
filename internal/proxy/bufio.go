package proxy

import (
	"bufio"
	"io"
)

// newBufReaderFromConn creates a *bufio.Reader from any io.Reader.
func newBufReaderFromConn(r io.Reader) *bufio.Reader {
	return bufio.NewReader(r)
}
