package local

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
)

type TerminalResult struct {
	Conn *os.File
	cmd  *exec.Cmd
}

func (self TerminalResult) Close() error {
	if self.Conn != nil {
		err := self.Conn.Close()
		if err != nil {
			slog.Debug("terminal result close", "error", err.Error())
		}
	}
	if self.cmd != nil && self.cmd.Process != nil {
		var err error
		if self.cmd.Cancel != nil {
			err = self.cmd.Cancel()
		} else {
			err = self.cmd.Process.Kill()
		}
		if err != nil {
			slog.Debug("terminal result close", "error", err.Error())
		}
		err = self.cmd.Wait()
		if err != nil {
			slog.Debug("terminal result wait", "error", err.Error())
		}
		return err
	}
	return nil
}

func (self TerminalResult) Write(p []byte) (n int, err error) {
	return self.Conn.Write(p)
}

type readCloser struct {
	cmd   *Local
	Conn  io.ReadCloser
	stdin io.WriteCloser
}

func (self readCloser) Read(p []byte) (n int, err error) {
	return self.Conn.Read(p)
}

func (self readCloser) Close() error {
	_ = self.Conn.Close()
	if self.stdin != nil {
		_ = self.stdin.Close()
	}
	return self.cmd.Close()
}

func (self readCloser) Write(p []byte) (n int, err error) {
	if self.stdin == nil {
		return 0, io.ErrClosedPipe
	}
	return self.stdin.Write(p)
}
