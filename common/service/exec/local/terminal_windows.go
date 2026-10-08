//go:build windows

package local

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/creack/pty"
)

func (self *Local) RunInTerminal(size *pty.Winsize) (io.Reader, io.WriteCloser, error) {
	self.debug()
	if strings.EqualFold(filepath.Base(self.cmd.Path), "cmd.exe") {
		quiet := false
		for _, arg := range self.cmd.Args[1:] {
			if strings.EqualFold(arg, "/Q") {
				quiet = true
				break
			}
		}
		if !quiet {
			self.cmd.Args = append(self.cmd.Args[:1], append([]string{"/Q"}, self.cmd.Args[1:]...)...)
		}
	}
	stdoutReader, stdoutWriter := io.Pipe()
	stdinReader, stdinWriter := io.Pipe()
	stderrBuf := &bytes.Buffer{}
	self.cmd.Stdin = stdinReader
	self.cmd.Stdout = stdoutWriter
	self.cmd.Stderr = io.MultiWriter(stdoutWriter, stderrBuf)
	if err := self.cmd.Start(); err != nil {
		_ = stdoutWriter.CloseWithError(err)
		_ = stdinReader.CloseWithError(err)
		_ = stdinWriter.CloseWithError(err)
		return nil, nil, err
	}
	go func() {
		err := self.cmd.Wait()
		if err != nil {
			err = fmt.Errorf("%s: %s", err.Error(), stderrBuf.String())
			slog.Debug("run command wait", "err", err)
			_ = stdoutWriter.CloseWithError(err)
		} else {
			_ = stdoutWriter.Close()
		}
		_ = stdinReader.Close()
	}()
	terminalOutput, terminalInput := self.windowsTerminalStreams(stdoutReader, stdinWriter)
	return terminalOutput, readCloser{
		cmd:   self,
		Conn:  stdoutReader,
		stdin: terminalInput,
	}, nil
}
