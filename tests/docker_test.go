package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/service/docker"
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

func TestContainerRemove(t *testing.T) {
	sdk, _ := docker.NewDockerClient()
	err := sdk.Client.ContainerStop(context.Background(), "phpmyadmin", container.StopOptions{})
	err = sdk.Client.ContainerRemove(context.Background(), "phpmyadmin", types.ContainerRemoveOptions{})
	fmt.Printf("%v \n", err)

}

type progressDetail struct {
	Id             string `json:"id"`
	Status         string `json:"status"`
	ProgressDetail struct {
		Current float64 `json:"current"`
		Total   float64 `json:"total"`
	} `json:"progressDetail"`
}

type pullImageProgress struct {
	Downloading float64
	Extracting  float64
}

func TestPullImage(t *testing.T) {
	sdk, _ := docker.NewDockerClient()
	//尝试拉取镜像
	reader, err := sdk.Client.ImagePull(context.Background(), "phpmyadmin", types.ImagePullOptions{})
	if err != nil {
		fmt.Printf("%v \n", err)
	}

	var progress map[string]*pullImageProgress
	progress = make(map[string]*pullImageProgress)

	out := bufio.NewReader(reader)
	for {
		str, err := out.ReadString('\n')
		if err == io.EOF {
			break
		} else {
			pd := &progressDetail{}
			json.Unmarshal([]byte(str), pd)
			if pd.Status == "Pulling fs layer" {
				progress[pd.Id] = &pullImageProgress{
					Extracting:  0,
					Downloading: 0,
				}
			}
			if pd.ProgressDetail.Total > 0 && pd.Status == "Downloading" {
				progress[pd.Id].Downloading = math.Floor((pd.ProgressDetail.Current / pd.ProgressDetail.Total) * 100)
			}
			if pd.ProgressDetail.Total > 0 && pd.Status == "Extracting" {
				progress[pd.Id].Extracting = math.Floor((pd.ProgressDetail.Current / pd.ProgressDetail.Total) * 100)
			}
			if pd.Status == "Download complete" {
				progress[pd.Id].Downloading = 100
			}
			if pd.Status == "Pull complete" {
				progress[pd.Id].Extracting = 100
			}

			fmt.Printf("%v \n", progress)
		}
	}
}

func TestCreateContainer(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	builder := sdk.GetContainerCreateBuilder()
	builder.WithImage("portainer/portainer-ce:latest")
	builder.WithContainerName("portainer")
	//builder.WithEnv("PMA_HOST", "localmysql")
	builder.WithPort("8000", "8000")
	builder.WithPort("9000", "9000")
	//builder.WithLink("localmysql", "localmysql")
	builder.WithAlwaysRestart()
	builder.WithPrivileged()
	builder.WithVolume("/var/run/docker.sock", "/var/run/docker.sock")
	response, err := builder.Execute()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	fmt.Printf("%v \n", response.ID)
	err = sdk.Client.ContainerStart(context.Background(), response.ID, types.ContainerStartOptions{})
	if err != nil {
		fmt.Printf("%v \n", err)
	}

}

func TestGetContainer(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	item, err := sdk.ContainerByField("name", "dpanel-site-50-bDOrc2t6G5", "dpanel-system-48-ULI6AsL1Yw", "dpanel-app-47-xZvGQCce3o")
	//if err != nil {
	//	fmt.Printf("%v \n", err)
	//	return
	//}
	//fmt.Printf("%v \n", item)

	item, err = sdk.ContainerByField("publish", "80", "9000")
	fmt.Printf("%v \n", item)
}

func TestGetContainerLog(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	filter := filters.NewArgs()
	filter.Add("desired-state", "running")
	filter.Add("desired-state", "shutdown")
	filter.Add("desired-state", "accepted")
	task, err := sdk.Client.TaskList(context.Background(), types.TaskListOptions{
		Filters: filter,
	})
	fmt.Printf("%v \n", task)
	return
	builder := sdk.GetContainerLogBuilder()
	builder.WithContainerId("0bf3c0b9f3d6")
	builder.WithTail(0)
	content, err := builder.Execute()
	fmt.Printf("%v \n", err)
	fmt.Printf("%v \n", content)
}

type progressStream struct {
	Stream string `json:"stream"`
}

type progressImageBuild struct {
	StepTotal   string `json:"stepTotal"`
	StepCurrent string `json:"stepCurrent"`
	Message     string `json:"message"`
}

func TestImageBuild(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	pg := progressImageBuild{}
	stream := progressStream{}
	str := "{\"stream\":\"Step 2/9 : RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories\"}"
	json.Unmarshal([]byte(str), &stream)

	field := strings.Fields(stream.Stream)
	if field != nil && field[0] == "Step" {
		step := strings.Split(field[1], "/")
		pg.StepTotal = step[1]
		pg.StepCurrent = step[0]
	}
	pg.Message = stream.Stream
	rs, _ := json.Marshal(pg)
	fmt.Printf("%v \n", string(rs))
	return
	builder := sdk.GetImageBuildBuilder()
	builder.WithZipFilePath("/Users/renchao/Workspace/open-system/artifact-lskypro/data2.zip")
	builder.WithDockerFileContent([]byte("adsfasdfsadf111111"))
	builder.Execute()

}

func TestLoginRegistry(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	auth, err := sdk.Client.RegistryLogin(context.Background(), registry.AuthConfig{
		Username:      "100009529522",
		Password:      "chaoren945RC",
		ServerAddress: "ccr.ccs.tencentyun.com",
	})
	fmt.Printf("%v \n", err)
	fmt.Printf("%v \n", auth)

	messageChan, errorChan := sdk.Client.Events(context.Background(), types.EventsOptions{})
	for true {
		select {
		case messaage := <-messageChan:
			fmt.Printf("%v \n", messaage)
			time.Sleep(time.Second)
		case err := <-errorChan:
			fmt.Printf("%v \n", err.Error())
			time.Sleep(time.Second)
		}
	}
}

func TestImage(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	result, _, err := sdk.Client.ImageInspectWithRaw(context.Background(), "dddd:latest")
	fmt.Printf("%v \n", result)
	result1, err := sdk.Client.ImageRemove(context.Background(), "phpmyadmin", types.ImageRemoveOptions{})
	fmt.Printf("%v \n", result1)
	fmt.Printf("%v \n", err)
}

func TestChan(t *testing.T) {
	messageQueue := make(chan string, 10)
	ctx := context.WithValue(context.Background(), "message", messageQueue)
	ctx, canel := context.WithCancel(ctx)
	messageQueue <- "abc"

	messageChan := ctx.Value("message").(chan string)

	select {
	case str := <-messageChan:
		fmt.Printf("%v \n", str)
	}
	fmt.Printf("%v \n", canel)
}

func TestCode(t *testing.T) {
	jsonStr := "{\"stream\":\"\\u001b[91mResolving dependencies through SAT\\n\\nDependency resolution completed in 0.000 seconds\\nYour lock file does not contain a compatible set of packages. Please run composer update.\\n\\n  Problem 1\\n    - alibabacloud/client is locked to version 1.5.32 and an update of this package was not requested.\\n    - alibabacloud/client 1.5.32 requires ext-simplexml * -\\u003e it is missing from your system. Install or enable PHP's simplexml extension.\\n  Problem 2\\n    - aws/aws-sdk-php is locked to version 3.261.13 and an update of this package was not requested.\\n    - aws/aws-sdk-php 3.261.13 requires ext-simplexml * -\\u003e it is missing from your system. Install or enable PHP's simplexml extension.\\n  Problem 3\\n    - intervention/image is locked to version 2.7.2 and an update of this package was not requested.\\n    - intervention/image 2.7.2 requires ext-fileinfo * -\\u003e it is missing from your system. Install or enable PHP's fileinfo extension.\\n  Problem 4\\n    - laravel/framework is locked to version v9.52.4 and an update of this package was not requested.\\n    - laravel/framework v9.52.4 requires ext-session * -\\u003e it is missing from your system. Install or enable PHP's session extension.\\n  Problem 5\\n    - league/flysystem-ftp is locked to version 3.10.3 and an update of this package was not requested.\\n    - league/flysystem-ftp 3.10.3 requires ext-ftp * -\\u003e it is missing from your system. Install or enable PHP's ftp extension.\\n  Problem 6\\n    - league/mime-type-detection is locked to version 1.11.0 and an update of this package was not requested.\\n    - league/mime-type-detection 1.11.0 requires ext-fileinfo * -\\u003e it is missing from your system. Install or enable PHP's fileinfo extension.\\n  Problem 7\\n    - nikic/php-parser is locked to version v4.15.4 and an update of this package was not requested.\\n    - nikic/php-parser v4.15.4 requires ext-tokenizer * -\\u003e it is missing from your system. Install or enable PHP's tokenizer extension.\\n  Problem 8\\n    - overtrue/qcloud-cos-client is locked to version 2.0.0 and an update of this package was not requested.\\n    - overtrue/qcloud-cos-client 2.0.0 requires ext-dom * -\\u003e it is missing from your system. Install or enable PHP's dom extension.\\n  Problem 9\\n    - psy/psysh is locked to version v0.11.12 and an update of this package was not requested.\\n    - psy/psysh v0.11.12 requires ext-tokenizer * -\\u003e it is missing from your system. Install or enable PHP's tokenizer extension.\\n  Problem 10\\n    - sabre/dav is locked to version 4.4.0 and an update of this package was not requested.\\n    - sabre/dav 4.4.0 requires ext-dom * -\\u003e it is missing from your system. Install or enable PHP's dom extension.\\n  Problem 11\\n    - sabre/xml is locked to version 2.2.5 and an update of this package was not requested.\\n    - sabre/xml 2.2.5 requires ext-dom * -\\u003e it is missing from your system. Install or enable PHP's dom extension.\\n  Problem 12\\n    - tijsverkoyen/css-to-inline-styles is locked to version 2.2.6 and an update of this package was not requested.\\n    - tijsverkoyen/css-to-inline-styles 2.2.6 requires ext-dom * -\\u003e it is missing from your system. Install or enable PHP's dom extension.\\n  Problem 13\\n    - phar-io/manifest is locked to version 2.0.3 and an update of this package was not requested.\\n    - phar-io/manifest 2.0.3 requires ext-dom * -\\u003e it is missing from your system. Install or enable PHP's dom extension.\\n  Problem 14\\n    - phpunit/php-code-coverage is locked to version 9.2.26 and an update of this package was not requested.\\n    - phpunit/php-code-coverage 9.2.26 requires ext-dom * -\\u003e it is missing from your system. Install or enable PHP's dom extension.\\n  Problem 15\\n    - phpunit/phpunit is locked to version 9.6.5 and an update of this package was not requested.\\n    - phpunit/phpunit 9.6.5 requires ext-dom * -\\u003e it is missing from your system. Install or enable PHP's dom extension.\\n  Problem 16\\n    - theseer/tokenizer is locked to version 1.2.1 and an update of this package was notrequested.\\n    - theseer/tokenizer 1.2.1 requires ext-dom * -\\u003e it is missing from your system. Install or enable PHP's dom extension.\\n  Problem 17\\n    - alibabacloud/client 1.5.32 requires ext-simplexml * -\\u003e it is missing from your system. Install or enable PHP's simplexml extension.\\n    - alibabacloud/green 1.8.958 requires alibabacloud/client ^1.5 -\\u003e satisfiable by alibabacloud/client[1.5.32].\\n    - alibabacloud/green is locked to version 1.8.958 and an update of this package was not requested.\\n\\nTo enable extensions, verify that they are enabled in your .ini files:\\n    - /etc/php82/php.ini\\n    - /etc/php82/conf.d/00_curl.ini\\n    - /etc/php82/conf.d/00_iconv.ini\\n    - /etc/php82/conf.d/00_mbstring.ini\\n    - /etc/php82/conf.d/00_openssl.ini\\n    - /etc/php82/conf.d/00_zip.ini\\n    - /etc/php82/conf.d/01_phar.ini\\nYou can also run `php --ini' in a terminal to see which files are used by PHP in CLI mode.\\nAlternatively, you can run Composer with `--ignore-platform-req=ext-simplexml --ignore-platform-req=ext-fileinfo --ignore-platform-req=ext-session --ignore-platform-req=ext-ftp --ignore-platform-req=ext-tokenizer --ignore-platform-req=ext-dom` to temporarily ignore these required extensions.\\n\\u001b[0m\"}"
	var val map[string]string
	err := json.Unmarshal([]byte(jsonStr), &val)
	fmt.Printf("%v \n", val)
	fmt.Printf("%v \n", err)
}
