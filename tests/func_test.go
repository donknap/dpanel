package tests

import (
	"fmt"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/stretchr/testify/assert"
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
