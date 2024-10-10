package compose

import (
	"context"
	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/donknap/dpanel/common/service/storage"
	"os"
	"path/filepath"
)

func WithYamlString(name string, yaml []byte) cli.ProjectOptionsFn {
	return func(options *cli.ProjectOptions) error {
		path := filepath.Join(storage.Local{}.GetComposePath(), name)
		err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
		if err != nil {
			return err
		}
		err = os.WriteFile(path, yaml, 0666)
		if err != nil {
			return err
		}
		options.ConfigPaths = append(options.ConfigPaths, path)
		return nil
	}
}

func WithYamlPath(path string) cli.ProjectOptionsFn {
	return func(options *cli.ProjectOptions) error {
		options.ConfigPaths = append(options.ConfigPaths, path)
		return nil
	}
}

func NewCompose(opts ...cli.ProjectOptionsFn) (*Wrapper, error) {
	// 自定义解析
	opts = append(opts,
		cli.WithExtension(ExtensionName, Ext{}),
		cli.WithExtension(ExtensionServiceName, ExtService{}),
	)
	options, err := cli.NewProjectOptions(
		[]string{},
		opts...,
	)
	if err != nil {
		return nil, err
	}

	project, err := options.LoadProject(context.Background())
	if err != nil {
		return nil, err
	}
	wrapper := &Wrapper{
		Project: project,
	}
	ext := Ext{}
	exists, err := project.Extensions.Get(ExtensionName, &ext)
	if err == nil && exists {
		wrapper.Ext = &ext
	}
	return wrapper, nil
}

type Wrapper struct {
	Project *types.Project
	Ext     *Ext
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
	return cmd
}
