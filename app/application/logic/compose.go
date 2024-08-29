package logic

import (
	"github.com/donknap/dpanel/common/service/exec"
	"os"
)

type ComposeTaskOption struct {
	SiteName    string
	Yaml        string
	DeleteImage bool
}

type Compose struct {
}

func (self Compose) Deploy(task *ComposeTaskOption) error {
	yamlFile, _ := os.CreateTemp("", "dpanel-compose")
	err := os.WriteFile(yamlFile.Name(), []byte(task.Yaml), 0666)
	if err != nil {
		return err
	}
	self.runCommand([]string{
		"-f",
		yamlFile.Name(),
		"-p",
		task.SiteName,
		"--progress",
		"tty",
		"up",
		"-d",
	})
	os.Remove(yamlFile.Name())
	return nil
}

func (self Compose) Destroy(task *ComposeTaskOption) error {
	yamlFile, err := self.getYamlFile(task.Yaml)
	if err != nil {
		return err
	}
	command := []string{
		"-f",
		yamlFile.Name(),
		"-p",
		task.SiteName,
		"--progress",
		"tty",
		"down",
	}
	if task.DeleteImage {
		command = append(command, "--rmi", "all")
	}
	self.runCommand(command)
	os.Remove(yamlFile.Name())
	return nil
}

func (self Compose) Ctrl(task *ComposeTaskOption, op string) error {
	yamlFile, err := self.getYamlFile(task.Yaml)
	if err != nil {
		return err
	}
	command := []string{
		"-f",
		yamlFile.Name(),
		"-p",
		task.SiteName,
		"--progress",
		"tty",
		op,
	}
	self.runCommand(command)
	os.Remove(yamlFile.Name())
	return nil
}

func (self Compose) Ls(projectName string) string {
	command := []string{
		"ls",
		"--format", "json",
		"--all",
	}
	if projectName != "" {
		command = append(command, "--filter", "name="+projectName)
	}
	return exec.Command{}.RunWithOut(&exec.RunCommandOption{
		CmdName: "docker",
		CmdArgs: append([]string{
			"compose",
		}, command...),
	})
}

func (self Compose) Kill() error {
	return exec.Command{}.Kill()
}

func (self Compose) runCommand(command []string) {
	exec.Command{}.RunInTerminal(&exec.RunCommandOption{
		CmdName: "docker",
		CmdArgs: append([]string{
			"compose",
		}, command...),
	})
}

func (self Compose) getYamlFile(yamlContent string) (*os.File, error) {
	yamlFile, _ := os.CreateTemp("", "dpanel-compose")
	err := os.WriteFile(yamlFile.Name(), []byte(yamlContent), 0666)
	if err != nil {
		return nil, err
	}
	return yamlFile, nil
}
