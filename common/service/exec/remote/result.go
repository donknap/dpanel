package remote

import (
	"golang.org/x/crypto/ssh"
	"io"
)

type readCloser struct {
	buffer  io.Reader
	session *ssh.Session
}

func (self *readCloser) Read(p []byte) (n int, err error) {
	return self.buffer.Read(p)
}

func (self *readCloser) Close() error {
	return self.session.Close()
}