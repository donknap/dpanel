package controller

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/backup"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/registry"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type ContainerBackup struct {
	controller.Abstract
}

func (self ContainerBackup) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id                string `json:"id" binding:"required"`
		EnableImage       bool   `json:"enableImage"`
		EnableVolume      bool   `json:"enableVolume"`
		EnableCommitImage bool   `json:"enableCommitImage"`
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

	suffix := time.Now().Format(function.YmdHis)
	backupRelTar := filepath.Join(containerInfo.Name, suffix+".tar.gz")
	backupTar := filepath.Join(storage.Local{}.GetBackupPath(), backupRelTar)

	backupRow := &entity.Backup{
		ContainerID: params.Id,
		Setting: &accessor.BackupSettingOption{
			BackupTargetType: docker.ContainerBackupTypeSnapshot,
			BackupTar:        backupRelTar,
			VolumePathList:   make([]string, 0),
			Status:           docker.ImageBuildStatusProcess,
		},
	}
	_ = dao.Backup.Save(backupRow)

	b, err := backup.New(
		backup.WithTarPathPrefix(containerInfo.Name),
		backup.WithPath(backupTar),
		backup.WithWriter(),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	progress := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeContainerBackup, backupRow.ID)).KeepAlive()
	defer func() {
		progress.Close()
	}()

	go func() {
		select {
		case <-progress.Done():
			_ = notice.Message{}.Info(".containerBackupFinish", "name", containerInfo.Name)
			if closeErr := b.Close(); closeErr != nil {
				backupRow.Setting.Status = docker.ImageBuildStatusError
				backupRow.Setting.Error = closeErr.Error()
				_ = dao.Backup.Save(backupRow)
			}
		}
	}()

	manifest := make([]backup.Manifest, 0)
	err = func() error {
		dockerVersion, err := docker.Sdk.Client.ServerVersion(progress.Context())
		if err != nil {
			return err
		}
		err = b.Writer.WriteConfigFile("version.json", dockerVersion)
		if err != nil {
			return err
		}
		item := backup.Manifest{}
		if params.EnableImage {
			imageId := containerInfo.Image
			imageName := containerInfo.Config.Image

			if params.EnableCommitImage {
				imageDetail := registry.GetImageTagDetail(containerInfo.Config.Image)
				imageName = fmt.Sprintf("%s-%s", imageDetail.Uri(), suffix)

				response, err := docker.Sdk.Client.ContainerCommit(progress.Context(), containerInfo.ID, container.CommitOptions{
					Reference: imageName,
				})
				if err != nil {
					return err
				}
				defer func() {
					_, err = docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, response.ID, image.RemoveOptions{
						Force: true,
					})
					if err != nil {
						slog.Warn("container backup remove image", "error", err)
					}
				}()

				imageId = response.ID
			}

			// 如果当前容器 commit 自己为新镜像时，需要在配置中更新名称
			containerInfo.Config.Image = imageName

			out, err := docker.Sdk.Client.ImageSave(progress.Context(), []string{
				imageName,
			})
			if err != nil {
				return err
			}
			imagePath, err := b.Writer.WriteBlobReader(imageId, out)
			if err != nil {
				return err
			}
			item.Image = imagePath
		}

		if !function.IsEmptyArray(containerInfo.Mounts) && params.EnableVolume {
			for _, mount := range containerInfo.Mounts {
				backupRow.Setting.VolumePathList = append(backupRow.Setting.VolumePathList, mount.Destination)

				out, info, err := docker.Sdk.Client.CopyFromContainer(progress.Context(), params.Id, mount.Destination)
				if err != nil {
					return err
				}
				if info.Size > 0 {
					path, err := b.Writer.WriteBlobReader(function.GetSha256([]byte(mount.Destination)), out)
					if err != nil {
						return err
					}
					item.Volume = append(item.Volume, path)
				}
			}
		}

		configPath, err := b.Writer.WriteBlobStruct(containerInfo)
		if err != nil {
			return err
		}
		item.Config = configPath

		manifest = append(manifest, item)
		return nil
	}()

	backupRow.Setting.Status = docker.ImageBuildStatusError
	if err != nil {
		backupRow.Setting.Error = err.Error()
	} else {
		err = b.Writer.WriteConfigFile("manifest.json", manifest)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		backupRow.Setting.Status = docker.ImageBuildStatusSuccess
		if info, err := os.Stat(backupTar); err == nil {
			backupRow.Setting.Size = info.Size()
		}
	}
	_ = dao.Backup.Save(backupRow)

	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self ContainerBackup) Restore(http *gin.Context) {
	type ParamsValidate struct {
		Id          int32 `json:"id" binding:"required"`
		EnableForce bool  `json:"enableForce"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error
	backupRow, err := dao.Backup.Where(dao.Backup.ID.Eq(params.Id)).First()
	if err != nil {
		self.JsonResponseWithError(http, define.ErrorMessageCommonDataNotFoundOrDeleted, 500)
		return
	}
	tarFilePath := filepath.Join(storage.Local{}.GetBackupPath(), backupRow.Setting.BackupTar)
	b, err := backup.New(
		backup.WithTarPathPrefix(backupRow.ContainerID),
		backup.WithPath(tarFilePath),
		backup.WithReader(),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = b.Close()
	}()
	manifest, err := b.Reader.Manifest()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	for _, item := range manifest {
		//config, err := b.Reader.ReadBlobsContent(item.Config)
		//if err != nil {
		//	self.JsonResponseWithError(http, err, 500)
		//	return
		//}
		//containerInfo := container.InspectResponse{}
		//err = json.Unmarshal(config, &containerInfo)
		//if err != nil {
		//	self.JsonResponseWithError(http, err, 500)
		//	return
		//}
		//imageOut, err := b.Reader.ReadBlobs(item.Image)
		//if err != nil {
		//	self.JsonResponseWithError(http, err, 500)
		//	return
		//}
		//imageLoadResponse, err := docker.Sdk.Client.ImageLoad(docker.Sdk.Ctx, imageOut)
		//if err != nil {
		//	self.JsonResponseWithError(http, err, 500)
		//	return
		//}
		//_, err = io.Copy(io.Discard, imageLoadResponse.Body)
		//if err != nil {
		//	self.JsonResponseWithError(http, err, 500)
		//	return
		//}
		//newName := "test" + function.GetRandomString(10)
		//_, err = docker.Sdk.Client.ContainerCreate(docker.Sdk.Ctx, containerInfo.Config, containerInfo.HostConfig, &network.NetworkingConfig{
		//	EndpointsConfig: containerInfo.NetworkSettings.Networks,
		//}, &v1.Platform{}, newName)
		//if err != nil {
		//	self.JsonResponseWithError(http, err, 500)
		//	return
		//}
		//_ = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, newName, container.StartOptions{})

		for _, volume := range item.Volume {
			reader, _ := b.Reader.ReadBlobs(volume)
			gzReader, _ := gzip.NewReader(reader)
			tarReader := tar.NewReader(gzReader)
			if options, err := docker.NewFileImport("/", docker.WithImportTar(tarReader)); err == nil {
				err = docker.Sdk.ContainerImport("testrJpU9qycGl", options)
			}
			gzReader.Close()
		}
	}
}

func (self ContainerBackup) GetList(http *gin.Context) {
	type ParamsValidate struct {
		ContainerId string `json:"containerId"`
		Page        int    `json:"page,default=1" binding:"omitempty,gt=0"`
		PageSize    int    `json:"pageSize" binding:"omitempty,gt=1"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	query := dao.Backup.Order(dao.Backup.ID.Desc())
	if params.ContainerId != "" {
		query = query.Where(dao.Backup.ContainerID.Like("%" + params.ContainerId + "%"))
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize > 0 {
		list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)
		self.JsonResponseWithoutError(http, gin.H{
			"total": total,
			"page":  params.Page,
			"list":  list,
		})
	} else {
		list, _ := query.Find()
		self.JsonResponseWithoutError(http, gin.H{
			"total": len(list),
			"page":  params.Page,
			"list":  list,
		})
	}
	return
}

func (self ContainerBackup) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	backupInfo, _ := dao.Backup.Where(dao.Backup.ID.In(params.Id...)).Find()
	for _, item := range backupInfo {
		_ = os.Remove(filepath.Join(storage.Local{}.GetBackupPath(), item.Setting.BackupTar))
		// 删除临时文件
		if tempFileList, err := filepath.Glob(filepath.Join(storage.Local{}.GetBackupPath(), filepath.Dir(item.Setting.BackupTar), "*.temp")); err == nil {
			for _, file := range tempFileList {
				_ = os.Remove(file)
			}
		}
	}
	_, err := dao.Backup.Where(dao.Backup.ID.In(params.Id...)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}
