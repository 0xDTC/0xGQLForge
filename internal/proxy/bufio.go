package proxy

import (
	"bufio"
	"bytes"
	"io"
)

// newBufReader creates a *bufio.Reader from a byte slice.
func newBufReader(data []byte) *bufio.Reader {
	return bufio.NewReader(bytes.NewReader(data))
}

// newBufReaderFromConn creates a *bufio.Reader from any io.Reader.
func newBufReaderFromConn(r io.Reader) *bufio.Reader {
	return bufio.NewReader(r)
}
