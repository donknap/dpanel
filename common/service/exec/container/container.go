package container

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/creack/pty"
	dockertypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/donknap/dpanel/common/service/exec"
)

func New(options ...Option) (exec.Executor, error) {
	self := &Container{}
	self.ctx, self.cancel = context.WithCancel(context.Background())
	for _, option := range options {
		if err := option(self); err != nil {
			_ = self.Close()
			return nil, err
		}
	}
	if self.client == nil || self.containerName == "" || len(self.command) == 0 || self.command[0] == "" {
		_ = self.Close()
		return nil, errors.New("Docker client, container and command are required")
	}
	return self, nil
}

func QuickRun(ctx context.Context, client *dockerclient.Client, containerName string, command ...string) ([]byte, error) {
	if len(command) == 0 {
		return nil, errors.New("container exec command is empty")
	}
	cmd, err := New(
		WithDockerClient(client), WithContainerName(containerName),
		WithCommandName(command[0]), WithArgs(command[1:]...), WithCtx(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer cmd.Close()
	return cmd.RunWithResult()
}

type Container struct {
	client        *dockerclient.Client
	containerName string
	command       []string
	env           []string
	dir           string
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
	response      *dockertypes.HijackedResponse
	option        containertypes.ExecOptions
	execID        string
}

func (self *Container) Run() error {
	_, err := self.RunWithResult()
	return err
}

func (self *Container) RunWithResult() ([]byte, error) {
	option := self.option
	option.Cmd = self.command
	option.Env = append(option.Env, self.env...)
	if self.dir != "" {
		option.WorkingDir = self.dir
	}
	option.Tty, option.AttachStdin, option.AttachStdout, option.AttachStderr = false, false, true, true
	created, err := self.client.ContainerExecCreate(self.ctx, self.containerName, option)
	if err != nil {
		return nil, err
	}
	response, err := self.client.ContainerExecAttach(self.ctx, created.ID, containertypes.ExecStartOptions{})
	if err != nil {
		return nil, err
	}
	self.mu.Lock()
	self.response = &response
	self.execID = created.ID
	self.mu.Unlock()
	stopClose := context.AfterFunc(self.ctx, response.Close)
	defer stopClose()
	defer response.Close()
	var output bytes.Buffer
	_, err = stdcopy.StdCopy(&output, &output, response.Reader)
	if err != nil {
		return output.Bytes(), err
	}
	status, err := self.client.ContainerExecInspect(self.ctx, created.ID)
	if err != nil {
		return output.Bytes(), err
	}
	if status.ExitCode != 0 {
		return output.Bytes(), fmt.Errorf("container command exited with code %d: %s", status.ExitCode, output.String())
	}
	return output.Bytes(), nil
}

func (self *Container) RunInPip() (exec.Pipe, error) {
	return self.runInPip(true)
}

func (self *Container) RunInReadPip() (io.ReadCloser, error) {
	return self.runInPip(false)
}

func (self *Container) runInPip(withInput bool) (*pipeReader, error) {
	option := self.option
	option.Cmd = self.command
	option.Env = append(option.Env, self.env...)
	if self.dir != "" {
		option.WorkingDir = self.dir
	}
	option.Tty, option.AttachStdin, option.AttachStdout, option.AttachStderr = false, withInput, true, true
	created, err := self.client.ContainerExecCreate(self.ctx, self.containerName, option)
	if err != nil {
		return nil, err
	}
	response, err := self.client.ContainerExecAttach(self.ctx, created.ID, containertypes.ExecStartOptions{})
	if err != nil {
		return nil, err
	}
	self.mu.Lock()
	self.response = &response
	self.execID = created.ID
	self.mu.Unlock()
	context.AfterFunc(self.ctx, response.Close)
	reader, writer := io.Pipe()
	go func() {
		defer response.Close()
		var stderr bytes.Buffer
		_, err := stdcopy.StdCopy(writer, io.MultiWriter(writer, &stderr), response.Reader)
		if err == nil {
			var status containertypes.ExecInspect
			status, err = self.client.ContainerExecInspect(self.ctx, created.ID)
			if err == nil && status.ExitCode != 0 {
				err = fmt.Errorf("container command exited with code %d: %s", status.ExitCode, stderr.String())
			}
		}
		_ = writer.CloseWithError(err)
	}()
	result := &pipeReader{PipeReader: reader, close: self.Close}
	if withInput {
		result.write = response.Conn
		result.closeWrite = response.CloseWrite
	}
	return result, nil
}

func (self *Container) RunInTerminal(size *pty.Winsize) (io.Reader, io.WriteCloser, error) {
	option := self.option
	option.Cmd = self.command
	option.Env = append(option.Env, self.env...)
	if self.dir != "" {
		option.WorkingDir = self.dir
	}
	option.Tty, option.AttachStdin, option.AttachStdout, option.AttachStderr = true, true, true, true
	if size != nil {
		option.ConsoleSize = &[2]uint{uint(size.Rows), uint(size.Cols)}
	}
	created, err := self.client.ContainerExecCreate(self.ctx, self.containerName, option)
	if err != nil {
		return nil, nil, err
	}
	response, err := self.client.ContainerExecAttach(self.ctx, created.ID, containertypes.ExecStartOptions{
		Tty: true, ConsoleSize: option.ConsoleSize,
	})
	if err != nil {
		return nil, nil, err
	}
	self.mu.Lock()
	self.response = &response
	self.execID = created.ID
	self.mu.Unlock()
	context.AfterFunc(self.ctx, response.Close)
	return response.Reader, &terminalWriter{Writer: response.Conn, close: self.Close}, nil
}

func (self *Container) ResizeTerminal(size *pty.Winsize) error {
	if size == nil || self.execID == "" {
		return nil
	}
	return self.client.ContainerExecResize(self.ctx, self.execID, containertypes.ResizeOptions{
		Height: uint(size.Rows), Width: uint(size.Cols),
	})
}

func (self *Container) Kill() error { return self.Close() }

func (self *Container) Close() error {
	self.cancel()
	self.mu.Lock()
	defer self.mu.Unlock()
	if self.response != nil {
		self.response.Close()
		self.response = nil
	}
	return nil
}

func (self *Container) String() string         { return strings.Join(self.command, " ") }
func (self *Container) AppendEnv(env []string) { self.env = append(self.env, env...) }
func (self *Container) AppendSystemEnv()       { self.env = append(self.env, os.Environ()...) }
func (self *Container) WorkDir(path string)    { self.dir = path }

type pipeReader struct {
	*io.PipeReader
	write      io.Writer
	closeWrite func() error
	close      func() error
}

func (self *pipeReader) Close() error {
	err := self.PipeReader.Close()
	return errors.Join(err, self.close())
}

func (self *pipeReader) Write(p []byte) (int, error) {
	if self.write == nil {
		return 0, io.ErrClosedPipe
	}
	return self.write.Write(p)
}
func (self *pipeReader) CloseWrite() error {
	if self.closeWrite == nil {
		return io.ErrClosedPipe
	}
	return self.closeWrite()
}

type terminalWriter struct {
	io.Writer
	close func() error
}

func (self *terminalWriter) Close() error { return self.close() }
