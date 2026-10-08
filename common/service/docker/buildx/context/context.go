package context

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		configPath:  filepath.Join(configRoot, sdk.Name, "config.toml"),
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
	result.Exists = target.Exists
	result.Proxy = target.Proxy
	if target.Exists {
		output, err := self.runBuildx("inspect", self.builderName)
		if err != nil {
			return result, fmt.Errorf("inspect buildx builder %q: %w", self.builderName, err)
		}
		result.Detail = string(output)
		result.Config, err = self.readTargetConfig(target)
		return result, err
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
	previousTarget, err := self.getTarget()
	if err != nil {
		return err
	}
	var previousConfig string
	if previousTarget.Exists {
		previousConfig, err = self.readTargetConfig(previousTarget)
		if err != nil {
			return err
		}
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
	if err = self.remove(true, true); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if cleanupErr := self.remove(true, true); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("clean up failed buildx builder: %w", cleanupErr))
			}
			if previousTarget.Exists {
				if restoreErr := self.create(previousConfig, previousTarget.Proxy); restoreErr != nil {
					err = errors.Join(err, fmt.Errorf("restore previous buildx builder: %w", restoreErr))
				}
			}
		}
	}()
	return self.create(*content, option.Proxy)
}

func (self *contextService) create(config, proxy string) (err error) {
	if _, err = self.run("docker", []string{"context", "create", self.name, "--description", self.description}); err != nil {
		return err
	}
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

func Remove(sdk *docker.Client, option RemoveOption) error {
	self, err := newContext(sdk)
	if err != nil {
		return err
	}
	return self.remove(option.Force, !option.ClearState)
}

func (self *contextService) remove(force, keepState bool) error {
	var removeErr error
	if _, err := self.runBuildx("inspect", self.builderName); err == nil {
		args := []string{"rm", self.builderName, "--keep-daemon", "--keep-state"}
		if force {
			args = append(args, "--force")
		}
		if _, err = self.runBuildx(args...); err != nil {
			if keepState {
				return fmt.Errorf("remove buildx builder %q: %w", self.builderName, err)
			}
			removeErr = fmt.Errorf("remove buildx builder %q: %w", self.builderName, err)
		}
	}
	if err := self.removeTarget(force, keepState); err != nil {
		if keepState {
			return err
		}
		removeErr = errors.Join(removeErr, err)
	}
	if err := self.removeDockerContext(force); err != nil {
		if keepState {
			return err
		}
		removeErr = errors.Join(removeErr, err)
	}
	if removeErr != nil {
		return removeErr
	}
	if keepState {
		return nil
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
