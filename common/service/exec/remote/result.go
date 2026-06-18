package remote

import (
	"io"

	"golang.org/x/crypto/ssh"
)

type readCloser struct {
	buffer  io.Reader
	writer  io.Writer
	closer  io.Closer
	session *ssh.Session
}

func (self *readCloser) Read(p []byte) (n int, err error) {
	return self.buffer.Read(p)
}

func (self *readCloser) Close() error {
	if self.closer != nil {
		_ = self.closer.Close()
	}
	if self.session == nil {
		return nil
	}
	return self.session.Close()
}

func (self *readCloser) Write(p []byte) (n int, err error) {
	if self.writer == nil {
		return 0, io.ErrClosedPipe
	}
	return self.writer.Write(p)
}
