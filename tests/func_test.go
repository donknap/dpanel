package tests

import (
	"fmt"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestGetImageTagDetail(t *testing.T) {
	asserter := assert.New(t)

	imageName := "abc.def:latest"
	detail := logic.Image{}.GetImageTagDetail(imageName)
	asserter.Equal(imageName, detail.ImageName)
	asserter.Equal(detail.Registry, "docker.io")
	asserter.Equal(detail.Namespace, "abc.def")
	asserter.Equal(detail.Version, "latest")

	imageName = "test:1.0.0"
	detail = logic.Image{}.GetImageTagDetail(imageName)

	asserter.Equal(imageName, detail.ImageName)
	asserter.Equal(detail.Registry, "docker.io")
	asserter.Equal(detail.Namespace, "test")
	asserter.Equal(detail.Version, "1.0.0")

	imageName = "test1/test:latest"
	detail = logic.Image{}.GetImageTagDetail(imageName)
	fmt.Printf("%v \n", detail)
	asserter.Equal(imageName, detail.ImageName)
	asserter.Equal(detail.Registry, "docker.io")
	asserter.Equal(detail.Namespace, "test1")

	imageName = "test.io/test1/test:latest"
	detail = logic.Image{}.GetImageTagDetail(imageName)
	fmt.Printf("%v \n", detail)
	asserter.Equal("test1/test:latest", detail.ImageName)
	asserter.Equal(detail.Registry, "test.io")
	asserter.Equal(detail.Namespace, "test1")

	imageName = "test.io/test1/test2/test"
	detail = logic.Image{}.GetImageTagDetail(imageName)
	fmt.Printf("%v \n", detail)
	asserter.Equal("test1/test2/test:latest", detail.ImageName)
	asserter.Equal(detail.Registry, "test.io")
	asserter.Equal(detail.Namespace, "test1")
	asserter.Equal(detail.Version, "latest")
}

func TestGetImageName(t *testing.T) {
	asserter := assert.New(t)

	newImageName := logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry:  "ccr.ccs.tencentyun.com",
		Name:      "mysql:latest",
		Namespace: "dpanel",
	})
	asserter.Equal(newImageName, "ccr.ccs.tencentyun.com/dpanel/mysql:latest")

	newImageName = logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry:  "ccr.ccs.tencentyun.com",
		Name:      "mysql/mysql:latest",
		Namespace: "dpanel",
	})
	asserter.Equal(newImageName, "ccr.ccs.tencentyun.com/dpanel/mysql:latest")

	newImageName = logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry:  "ccr.ccs.tencentyun.com",
		Name:      "mysql/mysql",
		Namespace: "dpanel",
	})
	asserter.Equal(newImageName, "ccr.ccs.tencentyun.com/dpanel/mysql:latest")

	newImageName = logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry:  "ccr.ccs.tencentyun.com",
		Name:      "mysql/mysql:1.0.0",
		Namespace: "dpanel",
	})
	asserter.Equal(newImageName, "ccr.ccs.tencentyun.com/dpanel/mysql:1.0.0")

	newImageName = logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry: "ccr.ccs.tencentyun.com",
		Name:     "mysql/mysql:1.0.0",
	})
	asserter.Equal(newImageName, "ccr.ccs.tencentyun.com/mysql/mysql:1.0.0")

	newImageName = logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry:  "ccr.ccs.tencentyun.com",
		Name:      "mysql:1.0.0",
		Namespace: "dpanel",
	})
	asserter.Equal(newImageName, "ccr.ccs.tencentyun.com/dpanel/mysql:1.0.0")
}

func TestSplitCommand(t *testing.T) {

	fmt.Printf("%v \n", filepath.FromSlash("/home/abc/def"))

	asserter := assert.New(t)

	cmd := "./dpanel server:start -f config.yaml"
	cmdArr := function.CommandSplit(cmd)
	asserter.Equal(cmdArr[3], "config.yaml")

	cmd = "sh -c \"./dpanel server:start -f config.yaml\""
	cmdArr = function.CommandSplit(cmd)
	asserter.Equal(cmdArr[2], "./dpanel server:start -f config.yaml")

	cmd = "/bin/sh -c ./dpanel server:start -f config.yaml"
	cmdArr = function.CommandSplit(cmd)
	asserter.Equal(cmdArr[5], "config.yaml")
}
