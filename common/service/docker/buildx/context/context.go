package context

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

type contextService struct {
	sdk         *docker.Client
	name        string
	builderName string
	description string
	configRoot  string
	configPath  string
}

type Result struct {
	Name   string
	Detail string
	Config string
	Proxy  string
	Exists bool
}

type CreateOption struct {
	Config *string
	Proxy  string
}

type RemoveOption struct {
	Force      bool
	ClearState bool
}

type removeOption struct {
	Force     bool
	Builder   bool
	Container bool
	Storage   bool
	Context   bool
}

func newContext(sdk *docker.Client) (*contextService, error) {
	if sdk == nil || sdk.Client == nil || sdk.DockerEnv == nil {
		return nil, errors.New("Docker client is required for buildx")
	}
	if !filepath.IsLocal(sdk.Name) || filepath.Base(sdk.Name) != sdk.Name {
		return nil, fmt.Errorf("invalid Docker environment name %q", sdk.Name)
	}
	configRoot := filepath.Join(storage.Local{}.GetStorageLocalPath(), "buildx")
	result := &contextService{
		sdk:         sdk,
		name:        fmt.Sprintf(define.DockerContextName, sdk.Name),
		builderName: fmt.Sprintf(define.DockerBuilderName, sdk.Name),
		description: fmt.Sprintf("Created by DPanel DO NOT DELETE!!! %s", function.Sha256Struct(sdk.DockerEnv)),
		configRoot:  configRoot,
		configPath:  filepath.Join(configRoot, "buildx_"+sdk.Name+".toml"),
	}
	if _, err := result.runBuildx("version"); err != nil {
		return nil, fmt.Errorf("Docker Buildx CLI is unavailable: %w", err)
	}
	return result, nil
}

func Get(sdk *docker.Client) (Result, error) {
	self, err := newContext(sdk)
	if err != nil {
		return Result{}, err
	}
	result := Result{Name: self.name}
	target, err := self.getTarget()
	if err != nil {
		return result, err
	}
	result.Proxy = target.Proxy
	if target.Exists {
		result.Config, err = self.readTargetConfig(target)
		if err != nil {
			return result, err
		}
		if !target.Running {
			if err := self.sdk.Client.ContainerStart(self.sdk.Ctx, target.ID, container.StartOptions{}); err != nil && !errdefs.IsNotModified(err) {
				slog.Warn("start buildkit container failed", "container", target.Name, "error", err)
				if removeErr := self.removeContainer(target, false); removeErr != nil {
					return result, errors.Join(fmt.Errorf("start buildkit container %q: %w", target.Name, err), fmt.Errorf("remove buildkit container %q: %w", target.Name, removeErr))
				}
				return result, nil
			}
			target, err = self.getTarget()
			if err != nil {
				return result, err
			}
			if !target.Running {
				if target.Exists {
					slog.Warn("buildkit container stopped after start", "container", target.Name)
					if err := self.removeContainer(target, false); err != nil {
						return result, fmt.Errorf("remove stopped buildkit container %q: %w", target.Name, err)
					}
				}
				return result, nil
			}
		}
		contextExists, err := self.dockerContextExists()
		if err != nil || !contextExists {
			return result, nil
		}
		output, err := self.runBuildx("inspect", self.builderName)
		if err != nil {
			return result, nil
		}
		result.Detail = string(output)
		result.Exists = true
		return result, nil
	}
	content, err := os.ReadFile(self.configPath)
	if errors.Is(err, os.ErrNotExist) {
		result.Config, err = self.defaultConfig()
		return result, err
	}
	if err != nil {
		return result, err
	}
	result.Config = string(content)
	return result, nil
}

func Create(sdk *docker.Client, option CreateOption) (err error) {
	self, err := newContext(sdk)
	if err != nil {
		return err
	}
	if err = self.ensureDockerContext(); err != nil {
		return err
	}
	target, err := self.getTarget()
	if err != nil {
		return err
	}
	builderExists, err := self.builderExists()
	if err != nil {
		return err
	}
	if builderExists && (!target.Exists || option.Config == nil) {
		if _, err = self.runBuildx("inspect", "--bootstrap", self.builderName); err != nil {
			return err
		}
		target, err = self.getTarget()
		if err != nil {
			return err
		}
		if !target.Exists {
			return fmt.Errorf("buildkit container %q is unavailable", target.Name)
		}
		if option.Config == nil {
			return nil
		}
	}
	if target.Exists {
		previousConfig, err := self.readTargetConfig(target)
		if err != nil {
			return err
		}
		if option.Config == nil || (*option.Config == previousConfig && option.Proxy == target.Proxy) {
			if !builderExists {
				return self.create(previousConfig, target.Proxy)
			}
			_, err = self.runBuildx("inspect", "--bootstrap", self.builderName)
			return err
		}
		previousProxy := target.Proxy
		if err = self.remove(removeOption{Force: true, Builder: builderExists, Container: true}); err != nil {
			return err
		}
		defer func() {
			if err == nil {
				return
			}
			if cleanupErr := self.remove(removeOption{Force: true, Builder: true, Container: true}); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("clean up failed buildx builder: %w", cleanupErr))
			}
			if restoreErr := self.create(previousConfig, previousProxy); restoreErr != nil {
				err = errors.Join(err, fmt.Errorf("restore previous buildx builder: %w", restoreErr))
			}
		}()
		return self.create(*option.Config, option.Proxy)
	}
	content := option.Config
	if content == nil {
		saved, readErr := os.ReadFile(self.configPath)
		switch {
		case readErr == nil:
			config := string(saved)
			content = &config
		case errors.Is(readErr, os.ErrNotExist):
			config, err := self.defaultConfig()
			if err != nil {
				return err
			}
			content = &config
		default:
			return readErr
		}
	}
	if builderExists {
		if err = self.remove(removeOption{Force: true, Builder: true}); err != nil {
			return err
		}
	}
	return self.create(*content, option.Proxy)
}

func (self *contextService) create(config, proxy string) (err error) {
	configDir := filepath.Dir(self.configPath)
	if err = os.MkdirAll(configDir, 0700); err != nil {
		return err
	}
	configFile, err := os.CreateTemp(configDir, ".buildkit-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(configFile.Name())
	if err = configFile.Chmod(0600); err != nil {
		_ = configFile.Close()
		return err
	}
	if _, err = configFile.WriteString(config); err != nil {
		_ = configFile.Close()
		return err
	}
	if err = configFile.Close(); err != nil {
		return err
	}
	args := []string{"create", "--name", self.builderName, "--driver", "docker-container", "--driver-opt", "network=host", "--buildkitd-config", configFile.Name()}
	if proxy != "" {
		args = append(args, "--driver-opt", "env.HTTP_PROXY="+proxy, "--driver-opt", "env.HTTPS_PROXY="+proxy)
	}
	args = append(args, "--bootstrap", self.name)
	if _, err = self.runBuildx(args...); err != nil {
		return err
	}
	if err = os.Rename(configFile.Name(), self.configPath); err != nil {
		return err
	}
	return nil
}

func (self *contextService) ensureDockerContext() error {
	exists, err := self.dockerContextExists()
	if err != nil || exists {
		return err
	}
	_, err = self.run("docker", []string{"context", "create", self.name, "--description", self.description, "--docker", "host=" + self.sdk.DockerEnv.GetSockName()})
	return err
}

func (self *contextService) dockerContextExists() (bool, error) {
	output, err := self.run("docker", []string{"context", "ls", "--format", "{{.Name}}"})
	if err != nil {
		return false, err
	}
	for _, name := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(name) == self.name {
			return true, nil
		}
	}
	return false, nil
}

func (self *contextService) builderExists() (bool, error) {
	output, err := self.runBuildx("ls", "--format", "{{.Name}}")
	if err != nil {
		return false, fmt.Errorf("list buildx builders: %w", err)
	}
	for _, name := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(name) == self.builderName {
			return true, nil
		}
	}
	return false, nil
}

func Remove(sdk *docker.Client, option RemoveOption) error {
	self, err := newContext(sdk)
	if err != nil {
		return err
	}
	return self.remove(removeOption{
		Force:     option.Force,
		Builder:   true,
		Container: true,
		Storage:   option.ClearState,
		Context:   true,
	})
}

func (self *contextService) remove(option removeOption) error {
	var removeErr error
	if option.Builder {
		exists, err := self.builderExists()
		if err != nil {
			if !option.Storage {
				return err
			}
			removeErr = errors.Join(removeErr, err)
		} else if exists {
			args := []string{"rm", self.builderName, "--keep-daemon", "--keep-state"}
			if option.Force {
				args = append(args, "--force")
			}
			if _, err = self.runBuildx(args...); err != nil {
				removeErr = errors.Join(removeErr, fmt.Errorf("remove buildx builder %q: %w", self.builderName, err))
				if !option.Storage {
					return removeErr
				}
			}
		}
	}
	if option.Container {
		if err := self.removeTarget(option.Force); err != nil {
			removeErr = errors.Join(removeErr, err)
			if !option.Storage {
				return removeErr
			}
		}
	}
	if option.Storage {
		if err := self.removeStorage(); err != nil {
			removeErr = errors.Join(removeErr, err)
		}
	}
	if option.Context {
		if err := self.removeDockerContext(option.Force); err != nil {
			removeErr = errors.Join(removeErr, err)
			if !option.Storage {
				return err
			}
		}
	}
	return removeErr
}

func (self *contextService) removeStorage() error {
	volumeName := "buildx_buildkit_" + self.builderName + "0_state"
	if err := self.sdk.Client.VolumeRemove(self.sdk.Ctx, volumeName, false); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	if err := function.SafeDelete(self.configRoot, filepath.Base(self.configPath)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	legacyConfig := filepath.Join(self.configRoot, self.sdk.Name, "config.toml")
	info, err := os.Lstat(legacyConfig)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	// These names can also be directories managed by the shared Docker CLI config.
	switch self.sdk.Name {
	case "buildx", "contexts", "cli-plugins", "plugins", "features", "trust", "certs.d":
		return function.SafeDelete(self.configRoot, filepath.Join(self.sdk.Name, "config.toml"))
	}
	return function.SafeDeleteAll(self.configRoot, self.sdk.Name)
}

func Prune(sdk *docker.Client) error {
	self, err := newContext(sdk)
	if err != nil {
		return err
	}
	return self.pruneTarget()
}

func (self *contextService) removeDockerContext(force bool) error {
	for attempt := 0; attempt < 2; attempt++ {
		output, err := self.run("docker", []string{"context", "ls", "--format", "{{.Name}}"})
		if err != nil {
			return err
		}
		exists := false
		for _, name := range strings.Split(string(output), "\n") {
			if strings.TrimSpace(name) == self.name {
				exists = true
				break
			}
		}
		if !exists {
			return nil
		}
		if attempt == 1 {
			return fmt.Errorf("Docker context %q still exists after removal", self.name)
		}
		args := []string{"context", "rm", self.name}
		if force {
			args = append(args, "--force")
		}
		if _, err := self.run("docker", args); err != nil {
			return err
		}
	}
	return nil
}

func (self *contextService) run(name string, args []string) ([]byte, error) {
	cmd, err := local.New(
		local.WithCommandName(name),
		local.WithArgs(args...),
		local.WithEnv(self.sdk.DockerEnv.CommandEnv()),
		local.WithQuiet(),
		local.WithCtx(self.sdk.Ctx),
		local.WithIndependentProcessGroup(),
		local.WithKillProcessGroupOnCancel(),
	)
	if err != nil {
		return nil, err
	}
	defer cmd.Close()
	return cmd.RunWithResult()
}
