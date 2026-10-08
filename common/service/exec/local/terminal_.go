//go:build !windows

package local

import (
	"io"

	"github.com/creack/pty"
)

func (self *Local) RunInTerminal(size *pty.Winsize) (io.Reader, io.WriteCloser, error) {
	self.debug()
	out, err := pty.StartWithSize(self.cmd, size)
	if err != nil {
		return nil, nil, err
	}
	self.terminalFile = out
	return out, TerminalResult{
		Conn: out,
		cmd:  self.cmd,
	}, nil
}
