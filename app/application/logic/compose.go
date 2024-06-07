package logic

import (
	"bufio"
	"fmt"
	"github.com/creack/pty"
	"gopkg.in/yaml.v3"
	"io"
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

type composeProgress struct {
	Step string `json:"step"`
}
type Compose struct {
}

type writer struct {
}

func (w *writer) Write(p []byte) (n int, err error) {
	fmt.Printf("%v \n", string(p))
	return len(p), nil
}

func (self Compose) GetYaml(yamlStr string) (*dockerComposeYamlV2, error) {
	yamlObj := &dockerComposeYamlV2{}
	err := yaml.Unmarshal([]byte(yamlStr), yamlObj)
	if err != nil {
		return nil, err
	}
	return yamlObj, nil
}

func (self Compose) Deploy(task *ComposeTask) (*composeProgress, error) {
	yamlFile, _ := os.CreateTemp("", "dpanel-compose")
	err := os.WriteFile(yamlFile.Name(), []byte(task.Yaml), 0666)
	if err != nil {
		return nil, err
	}
	defer os.Remove(yamlFile.Name())

	cmd := exec.Command("docker-compose", []string{
		"-f",
		yamlFile.Name(),
		"-p",
		task.SiteName,
		"--progress",
		"tty",
		"up",
		"-d",
	}...)
	f, err := pty.Start(cmd)

	myWrite := &writer{}
	io.Copy(myWrite, f)
	//progressOut, err := cmd.StderrPipe()
	//if err != nil {
	//	return nil, err
	//}
	//
	//cmd.Start()
	//result := &composeProgress{}
	//reader := bufio.NewReaderSize(progressOut, 8192)
	//for {
	//	line, _, err := reader.ReadLine()
	//	if err == io.EOF {
	//		return result, nil
	//	} else {
	//		fmt.Printf("%v \n", string(line))
	//	}
	//}
	//cmd.Wait()
	//
	return nil, nil
}

func (self Compose) Uninstall(task *ComposeTask) error {
	yamlFile, _ := os.CreateTemp("", "dpanel-compose")
	err := os.WriteFile(yamlFile.Name(), []byte(task.Yaml), 0666)
	if err != nil {
		return err
	}
	defer os.Remove(yamlFile.Name())

	command := []string{
		"-f",
		yamlFile.Name(),
		"-p",
		task.SiteName,
		"down",
	}
	if task.DeleteImage {
		command = append(command, "--rmi", "all")
	}
	cmd := exec.Command("docker-compose", command...)

	progressOut, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Start()
	reader := bufio.NewReaderSize(progressOut, 8192)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			return nil
		} else {
			fmt.Printf("%v \n", string(line))
		}
	}
	cmd.Wait()
	return nil
}
