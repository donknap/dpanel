package exec

import (
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
	if self.cmd != nil {
		err := self.cmd.Process.Kill()
		if err != nil {
			slog.Debug("terminal result close", "error", err.Error())
		} else {
			return self.cmd.Wait()
		}
	}
	return nil
}

func (self TerminalResult) Read(p []byte) (n int, err error) {
	return self.Conn.Read(p)
}
