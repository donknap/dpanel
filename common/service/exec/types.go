package exec

import (
	"io"

	"github.com/creack/pty"
)

type Executor interface {
	// Run 执行一条命令，不关心输出信息，只关心执行是否成功
	Run() error
	RunWithResult() ([]byte, error)
	RunInPip() (io.ReadCloser, error)
	RunInTerminal(size *pty.Winsize) (io.ReadCloser, error)
	Kill() error
	Close() error
	String() string
	AppendEnv(env []string)
	AppendSystemEnv()
	WorkDir(path string)
}
