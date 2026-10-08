//go:build !windows

package build

func buildxCommand(args ...string) (string, []string) {
	return "docker", append([]string{"buildx"}, args...)
}
