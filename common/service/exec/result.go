package exec

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type TerminalResult struct {
	Conn *os.File
	cmd  *exec.Cmd
}

func (self TerminalResult) Close() error {
	var errCmd error
	var errConn error
	if self.cmd != nil {
		errCmd = self.cmd.Process.Kill()
	}
	if self.Conn != nil {
		errConn = self.Conn.Close()
	}
	return errors.Join(errCmd, errConn)
}

func (self TerminalResult) Read(p []byte) (n int, err error) {
	fmt.Printf("%v \n", self.Conn)
	if self.Conn == nil {
		return 0, io.EOF
	}
	return self.Conn.Read(p)
}
