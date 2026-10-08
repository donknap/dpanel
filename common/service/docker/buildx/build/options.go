package build

import "github.com/docker/docker/api/types/registry"

type Options struct {
	RegistryAuth []*registry.AuthConfig
	WorkDir      string
	Annotation   []string
	BuildArg     []string
	CacheFrom    []string
	CacheTo      []string
	Labels       []string
	ExtraArgs    []string
	Outputs      []string
	Platforms    []string
	Secrets      []string
	Builder      string
	File         string
	Target       []Target
	NoCache      bool
	Pull         bool
	Provenance   *bool
	Push         bool
}

type Target struct {
	Target string
	Tags   []string
}
