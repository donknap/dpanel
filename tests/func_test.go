package tests

import (
	"fmt"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetImageTagDetail(t *testing.T) {
	assert := assert.New(t)

	imageName := "abc.def:latest"
	detail := logic.Image{}.GetImageTagDetail(imageName)
	assert.Equal(imageName, detail.ImageName)
	assert.Equal(detail.Registry, "docker.io")
	assert.Equal(detail.Namespace, "abc.def")
	assert.Equal(detail.Version, "latest")

	imageName = "test:1.0.0"
	detail = logic.Image{}.GetImageTagDetail(imageName)

	assert.Equal(imageName, detail.ImageName)
	assert.Equal(detail.Registry, "docker.io")
	assert.Equal(detail.Namespace, "test")
	assert.Equal(detail.Version, "1.0.0")

	imageName = "test1/test:latest"
	detail = logic.Image{}.GetImageTagDetail(imageName)
	fmt.Printf("%v \n", detail)
	assert.Equal(imageName, detail.ImageName)
	assert.Equal(detail.Registry, "docker.io")
	assert.Equal(detail.Namespace, "test1")

	imageName = "test.io/test1/test:latest"
	detail = logic.Image{}.GetImageTagDetail(imageName)
	fmt.Printf("%v \n", detail)
	assert.Equal("test1/test:latest", detail.ImageName)
	assert.Equal(detail.Registry, "test.io")
	assert.Equal(detail.Namespace, "test1")

	imageName = "test.io/test1/test2/test"
	detail = logic.Image{}.GetImageTagDetail(imageName)
	fmt.Printf("%v \n", detail)
	assert.Equal("test1/test2/test:latest", detail.ImageName)
	assert.Equal(detail.Registry, "test.io")
	assert.Equal(detail.Namespace, "test1")
	assert.Equal(detail.Version, "latest")
}

func TestGetImageName(t *testing.T) {
	assert := assert.New(t)

	newImageName := logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry:  "ccr.ccs.tencentyun.com",
		Name:      "mysql:latest",
		Namespace: "dpanel",
	})
	assert.Equal(newImageName, "ccr.ccs.tencentyun.com/dpanel/mysql:latest")

	newImageName = logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry:  "ccr.ccs.tencentyun.com",
		Name:      "mysql/mysql:latest",
		Namespace: "dpanel",
	})
	assert.Equal(newImageName, "ccr.ccs.tencentyun.com/dpanel/mysql:latest")

	newImageName = logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry:  "ccr.ccs.tencentyun.com",
		Name:      "mysql/mysql",
		Namespace: "dpanel",
	})
	assert.Equal(newImageName, "ccr.ccs.tencentyun.com/dpanel/mysql:latest")

	newImageName = logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry:  "ccr.ccs.tencentyun.com",
		Name:      "mysql/mysql:1.0.0",
		Namespace: "dpanel",
	})
	assert.Equal(newImageName, "ccr.ccs.tencentyun.com/dpanel/mysql:1.0.0")

	newImageName = logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry: "ccr.ccs.tencentyun.com",
		Name:     "mysql/mysql:1.0.0",
	})
	assert.Equal(newImageName, "ccr.ccs.tencentyun.com/mysql/mysql:1.0.0")

	newImageName = logic.Image{}.GetImageName(&logic.ImageNameOption{
		Registry:  "ccr.ccs.tencentyun.com",
		Name:      "mysql:1.0.0",
		Namespace: "dpanel",
	})
	assert.Equal(newImageName, "ccr.ccs.tencentyun.com/dpanel/mysql:1.0.0")
	fmt.Printf("%v \n", newImageName)
}
