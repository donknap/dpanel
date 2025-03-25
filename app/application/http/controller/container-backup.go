package controller

import (
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/backup"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"path/filepath"
	"strings"
)

type ContainerBackup struct {
	controller.Abstract
}

func (self ContainerBackup) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id          string `json:"id" binding:"required"`
		EnableImage bool   `json:"enableImage"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error

	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.Id)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	manifest := &backup.Manifest{}
	if dockerVersion, err := docker.Sdk.Client.ServerVersion(docker.Sdk.Ctx); err == nil {
		manifest.ServerVersion = dockerVersion
	}

	suffix := "test" // time.Now().Format(function.YmdHis)
	backupPath := filepath.Join(storage.Local{}.GetBackupPath(), containerInfo.Name, suffix+".tar.gz")

	b, err := backup.New(
		backup.WithTarPathPrefix(containerInfo.Name),
		backup.WithPath(backupPath),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		b.Close()
	}()

	manifest.Config, err = b.Write.WriteBlobStruct(containerInfo)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if params.EnableImage {
		response, err := docker.Sdk.Client.ContainerCommit(docker.Sdk.Ctx, containerInfo.ID, container.CommitOptions{
			Reference: fmt.Sprintf("%s:%s", strings.TrimLeft(containerInfo.Name, "/"), suffix),
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		out, err := docker.Sdk.Client.ImageSave(docker.Sdk.Ctx, []string{
			response.ID,
		})
		defer func() {
			_ = out.Close()
		}()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		manifest.Image, err = b.Write.WriteBlobReader(response.ID, out)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if !function.IsEmptyArray(containerInfo.Mounts) {
		for _, mount := range containerInfo.Mounts {
			out, info, err := docker.Sdk.Client.CopyFromContainer(docker.Sdk.Ctx, params.Id, mount.Destination)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			if info.Size > 0 {
				path, err := b.Write.WriteBlobReader(function.GetSha256([]byte(mount.Destination)), out)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
				manifest.Volume = append(manifest.Volume, path)
			}
		}
	}
	err = b.Write.WriteManifest(manifest)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	//manifest := logic.ContainerBackupManifest{}
	//tarWriter.WriteHeader(&tar.Header{
	//	Name:    "manifest.json",
	//	Size:    int64(len(content)),
	//	Mode:    int64(os.ModePerm),
	//	ModTime: time.Now(),
	//})
	//fmt.Printf("%v \n", backupPath)
	//fmt.Printf("%v \n", containerInfo)
	self.JsonSuccessResponse(http)
	return
}
