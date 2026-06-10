package remote

import (
	"io"

	"golang.org/x/crypto/ssh"
)

type readCloser struct {
	buffer  io.Reader
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
