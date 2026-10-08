//go:build !windows

package context

func (self *contextService) runBuildx(args ...string) ([]byte, error) {
	return self.run("docker", append([]string{"buildx"}, args...))
}
