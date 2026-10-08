package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	exec2 "os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
)

type Executor struct {
	ctx        context.Context
	cancel     context.CancelFunc
	options    *Options
	env        []string
	processDir string
	mu         sync.Mutex
	started    bool
	current    exec.Executor
}

func NewExecutor(ctx context.Context, options *Options, env, dockerEnv []string) (*Executor, error) {
	ctx, cancel := context.WithCancel(ctx)
	result := &Executor{
		ctx:     ctx,
		cancel:  cancel,
		options: options,
		env:     append(append([]string{}, env...), dockerEnv...),
	}
	if options.Push {
		for _, auth := range options.RegistryAuth {
			cmd := exec2.CommandContext(ctx, "docker", "login", auth.ServerAddress, "-u", auth.Username, "--password-stdin")
			cmd.Env = dockerEnv
			cmd.Stdin = strings.NewReader(auth.Password + "\n")
			if err := cmd.Run(); err != nil {
				cancel()
				return nil, fmt.Errorf("docker login to registry %q: %w", auth.ServerAddress, err)
			}
		}
	}
	return result, nil
}

func (self *Executor) begin() error {
	self.mu.Lock()
	defer self.mu.Unlock()
	if self.started {
		return errors.New("buildx command has already started")
	}
	if err := self.ctx.Err(); err != nil {
		return err
	}
	self.started = true
	return nil
}

func (self *Executor) Run() error {
	if err := self.begin(); err != nil {
		return err
	}
	return self.run(io.Discard)
}

func (self *Executor) RunWithResult() ([]byte, error) {
	if err := self.begin(); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := self.run(&output); err != nil {
		return output.Bytes(), err
	}
	return output.Bytes(), nil
}

func (self *Executor) RunInReadPip() (io.ReadCloser, error) {
	if err := self.begin(); err != nil {
		return nil, err
	}
	reader, writer := io.Pipe()
	go func() {
		_ = writer.CloseWithError(self.run(writer))
	}()
	return reader, nil
}

type outputPipe struct {
	io.ReadCloser
}

func (self outputPipe) Write(data []byte) (int, error) { return 0, io.ErrClosedPipe }
func (self outputPipe) CloseWrite() error              { return nil }

func (self *Executor) RunInPip() (exec.Pipe, error) {
	reader, err := self.RunInReadPip()
	if err != nil {
		return nil, err
	}
	return outputPipe{ReadCloser: reader}, nil
}

type outputWriter struct{}

func (outputWriter) Write(data []byte) (int, error) { return 0, io.ErrClosedPipe }
func (outputWriter) Close() error                   { return nil }

func (self *Executor) RunInTerminal(_ *pty.Winsize) (io.Reader, io.WriteCloser, error) {
	reader, err := self.RunInReadPip()
	if err != nil {
		return nil, nil, err
	}
	return reader, outputWriter{}, nil
}

func (self *Executor) ResizeTerminal(size *pty.Winsize) error {
	self.mu.Lock()
	current := self.current
	self.mu.Unlock()
	if current == nil {
		return nil
	}
	return current.ResizeTerminal(size)
}

func (self *Executor) Kill() error  { return self.Close() }
func (self *Executor) Close() error { self.cancel(); return nil }
func (self *Executor) String() string {
	return "docker buildx build"
}

func (self *Executor) AppendEnv(env []string) {
	self.mu.Lock()
	defer self.mu.Unlock()
	if !self.started {
		self.env = append(self.env, env...)
	}
}

func (self *Executor) AppendSystemEnv() { self.AppendEnv(os.Environ()) }

func (self *Executor) WorkDir(path string) {
	self.mu.Lock()
	defer self.mu.Unlock()
	if !self.started {
		self.processDir = path
	}
}

func (self *Executor) runCommand(output io.Writer, args ...string) error {
	name, commandArgs := buildxCommand(args...)
	cmd, err := local.New(
		local.WithCommandName(name),
		local.WithArgs(commandArgs...),
		local.WithEnv(self.env),
		local.WithDir(self.processDir),
		local.WithQuiet(),
		local.WithCtx(self.ctx),
		local.WithIndependentProcessGroup(),
		local.WithKillProcessGroupOnCancel(),
	)
	if err != nil {
		return err
	}
	self.mu.Lock()
	self.current = cmd
	self.mu.Unlock()
	defer func() {
		self.mu.Lock()
		self.current = nil
		self.mu.Unlock()
		_ = cmd.Close()
	}()
	pipe, err := cmd.RunInReadPip()
	if err != nil {
		return err
	}
	defer pipe.Close()
	_, err = io.Copy(output, pipe)
	return err
}

func (self *Executor) run(output io.Writer) error {
	if _, err := io.WriteString(output, "Starting ...\n"); err != nil {
		return err
	}
	if self.options.Builder != "" {
		if err := self.runCommand(io.Discard, "inspect", self.options.Builder); err == nil {
			if err := self.runCommand(io.Discard, "inspect", "--bootstrap", self.options.Builder); err != nil {
				return err
			}
		}
	} else if err := self.runCommand(io.Discard, "inspect", "--bootstrap"); err != nil {
		return err
	}
	for _, target := range self.options.Target {
		name := target.Target
		if name == "" {
			name = "default"
		}
		if _, err := fmt.Fprintf(output, "Building target: %s ...\n", name); err != nil {
			return err
		}
		if err := self.runTarget(output, target, name); err != nil {
			_, _ = fmt.Fprintf(output, "Error: Build failed for target %s\n", name)
			return err
		}
	}
	return nil
}

func (self *Executor) runTarget(output io.Writer, target Target, name string) error {
	metadata, err := os.CreateTemp("", "dpanel_build_*")
	if err != nil {
		return err
	}
	defer os.Remove(metadata.Name())
	if err := metadata.Close(); err != nil {
		return err
	}
	args := []string{"build"}
	if self.options.Builder != "" {
		args = append(args, "--builder", self.options.Builder)
	}
	args = append(args, "--progress", "plain", "--metadata-file", metadata.Name())
	if self.options.Pull {
		args = append(args, "--pull")
	}
	if self.options.Push {
		if self.options.Provenance != nil {
			args = append(args, fmt.Sprintf("--provenance=%t", *self.options.Provenance))
		}
		if len(self.options.Outputs) > 0 {
			for _, value := range self.options.Outputs {
				args = append(args, "--output", value)
			}
		} else {
			args = append(args, "--push")
		}
	} else {
		args = append(args, "--load")
	}
	if self.options.NoCache {
		args = append(args, "--no-cache")
	}
	if self.options.File != "" {
		args = append(args, "-f", self.options.File)
	}
	if target.Target != "" {
		args = append(args, "--target", target.Target)
	}
	for _, tag := range target.Tags {
		args = append(args, "-t", tag)
	}
	for _, items := range []struct {
		flag   string
		values []string
	}{
		{"--build-arg", self.options.BuildArg},
		{"--cache-from", self.options.CacheFrom},
		{"--cache-to", self.options.CacheTo},
		{"--label", self.options.Labels},
		{"--annotation", self.options.Annotation},
		{"--platform", self.options.Platforms},
		{"--secret", self.options.Secrets},
	} {
		for _, value := range items.values {
			args = append(args, items.flag, value)
		}
	}
	args = append(args, self.options.ExtraArgs...)
	args = append(args, self.options.WorkDir)
	if err := self.runCommand(output, args...); err != nil {
		return err
	}
	content, err := os.ReadFile(metadata.Name())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "DPANEL_BUILD_RESULT|%s|%s\n", name, content)
	return err
}
