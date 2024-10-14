package compose

import "github.com/compose-spec/compose-go/v2/cli"

func WithYamlPath(path string) cli.ProjectOptionsFn {
	return func(options *cli.ProjectOptions) error {
		options.ConfigPaths = append(options.ConfigPaths, path)
		return nil
	}
}
