package compose

import (
	"github.com/compose-spec/compose-go/v2/types"
	"os"
	"path/filepath"
)

type Wrapper struct {
	Project *types.Project
}

// 区别于 Project.GetService 方法，此方法会将扩展信息一起返回
func (self Wrapper) GetService(name string) (types.ServiceConfig, ExtService, error) {
	service, err := self.Project.GetService(name)
	if err != nil {
		return types.ServiceConfig{}, ExtService{}, err
	}

	ext := ExtService{}
	exists, err := service.Extensions.Get(ExtensionServiceName, &ext)
	if err == nil && exists {
		return service, ext, nil
	}
	return service, ExtService{}, nil
}

func (self Wrapper) GetBaseCommand() []string {
	cmd := make([]string, 0)
	for _, file := range self.Project.ComposeFiles {
		cmd = append(cmd, "-f", file)
	}
	cmd = append(cmd, "-p", self.Project.Name)
	for _, envFileName := range []string{
		".env",
	} {
		envFilePath := filepath.Join(self.Project.WorkingDir, envFileName)
		_, err := os.Stat(envFilePath)
		if err == nil {
			cmd = append(cmd, "--env-file", envFilePath)
		}
	}
	return cmd
}
