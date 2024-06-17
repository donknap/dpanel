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

	imageName = "test:latest"
	detail = logic.Image{}.GetImageTagDetail(imageName)

	assert.Equal(imageName, detail.ImageName)
	assert.Equal(detail.Registry, "docker.io")
	assert.Equal(detail.Namespace, "test")

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

	imageName = "test.io/test1/test2/test:latest"
	detail = logic.Image{}.GetImageTagDetail(imageName)
	fmt.Printf("%v \n", detail)
	assert.Equal("test1/test2/test:latest", detail.ImageName)
	assert.Equal(detail.Registry, "test.io")
	assert.Equal(detail.Namespace, "test1")
}
