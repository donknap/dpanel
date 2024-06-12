package logic

import (
	"github.com/creack/pty"
	"github.com/donknap/dpanel/common/service/docker"
	"gopkg.in/yaml.v3"
	"io"
	"log/slog"
	"os"
	"os/exec"
)

type dockerComposeYamlV2 struct {
	Service map[string]struct {
		Image string `yaml:"image"`
		Build string `yaml:"build"`
	} `yaml:"service"`
}

type ComposeTask struct {
	SiteName    string
	Yaml        string
	DeleteImage bool
}

type writer struct {
}

func (self *writer) Write(p []byte) (n int, err error) {
	docker.QueueDockerComposeMessage <- string(p)
	return len(p), nil
}

type Compose struct {
}

func (self Compose) GetYaml(yamlStr string) (*dockerComposeYamlV2, error) {
	yamlObj := &dockerComposeYamlV2{}
	err := yaml.Unmarshal([]byte(yamlStr), yamlObj)
	if err != nil {
		return nil, err
	}
	return yamlObj, nil
}

func (self Compose) Deploy(task *ComposeTask) error {
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

func (self Compose) Destroy(task *ComposeTask) error {
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

func (self Compose) Ctrl(task *ComposeTask, op string) error {
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

func (self Compose) Ls(projectName string) error {
	command := []string{
		"ls",
		"--filter",
		"name=" + projectName,
	}
	self.runCommand(command)
	return nil
}

func (self Compose) runCommand(command []string) {
	myWrite := &writer{}

	cmd := exec.Command("docker-compose", command...)
	out, err := pty.Start(cmd)
	if err != nil {
		slog.Debug("docker-compose up", err.Error())
	}
	io.Copy(myWrite, out)
}

func (self Compose) getYamlFile(yamlContent string) (*os.File, error) {
	yamlFile, _ := os.CreateTemp("", "dpanel-compose")
	err := os.WriteFile(yamlFile.Name(), []byte(yamlContent), 0666)
	if err != nil {
		return nil, err
	}
	return yamlFile, nil
}
