package controller

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/backup"
	"github.com/donknap/dpanel/common/service/docker/imports"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type ContainerBackup struct {
	controller.Abstract
}

func (self ContainerBackup) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id                string   `json:"id" binding:"required"`
		EnableImage       bool     `json:"enableImage"`
		EnableCommitImage bool     `json:"enableCommitImage"`
		Volume            []string `json:"volume"`
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
	backupTime := time.Now().Format(define.DateYmdHis)
	suffix := fmt.Sprintf("dpanel-%s-%s", strings.TrimLeft(containerInfo.Name, "/"), backupTime)
	backupRelTar := filepath.Join(containerInfo.Name, suffix+".snapshot")
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

	info := backup.Info{}
	info.Docker, err = docker.Sdk.Client.ServerVersion(progress.Context())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	manifest := make([]backup.Manifest, 0)
	err = func() error {

		item := backup.Manifest{}
		if params.EnableImage {
			imageId := containerInfo.Image
			imageName := containerInfo.Config.Image

			if params.EnableCommitImage {
				imageDetail := function.ImageTag(containerInfo.Config.Image)
				imageName = fmt.Sprintf("%s-%s", imageDetail.Uri(), backupTime)

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

		if !function.IsEmptyArray(containerInfo.Mounts) {
			for _, mount := range containerInfo.Mounts {
				if !function.IsEmptyArray(params.Volume) && !function.InArray(params.Volume, mount.Destination) {
					continue
				}
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

		if containerInfo.NetworkSettings != nil && !function.IsEmptyMap(containerInfo.NetworkSettings.Networks) {
			for name := range containerInfo.NetworkSettings.Networks {
				if info, err := docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, name, network.InspectOptions{}); err == nil {
					configPath, err = b.Writer.WriteBlobStruct(info)
					if err != nil {
						return err
					}
					item.Network = append(item.Network, configPath)
				}
			}
		}
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

	info.Backup = backupRow
	err = b.Writer.WriteConfigFile("info.json", info)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"path": backupTar,
	})
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
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
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
		config, err := b.Reader.ReadBlobsContent(item.Config)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		containerInfo := container.InspectResponse{}
		err = json.Unmarshal(config, &containerInfo)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if _, err = docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, containerInfo.Config.Image); err != nil {
			if item.Image != "" {
				imageOut, err := b.Reader.ReadBlobs(item.Image)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
				imageLoadResponse, err := docker.Sdk.Client.ImageLoad(docker.Sdk.Ctx, imageOut)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
				_, err = io.Copy(io.Discard, imageLoadResponse.Body)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
				if _, err = docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, containerInfo.Config.Image); err != nil {
					self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerBackupRestoreImportImageFailed), 500)
					return
				}
			} else {
				out, err := docker.Sdk.Client.ImagePull(docker.Sdk.Ctx, containerInfo.Config.Image, image.PullOptions{})
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
				_, err = io.Copy(io.Discard, out)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
				_ = out.Close()
			}
		}

		newContainerName := containerInfo.Name
		if _, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, newContainerName); err != nil {
			var networkCreate []network.Inspect
			if !function.IsEmptyArray(item.Network) {
				for _, s := range item.Network {
					networkConfigContent, err := b.Reader.ReadBlobsContent(s)
					if err != nil {
						self.JsonResponseWithError(http, err, 500)
						return
					}
					networkInfo := network.Inspect{}
					err = json.Unmarshal(networkConfigContent, &networkInfo)
					if err != nil {
						self.JsonResponseWithError(http, err, 500)
						return
					}
					if _, err := docker.Sdk.Client.NetworkInspect(docker.Sdk.Ctx, networkInfo.Name, network.InspectOptions{}); err != nil {
						networkCreate = append(networkCreate, networkInfo)
						_, err = docker.Sdk.Client.NetworkCreate(docker.Sdk.Ctx, networkInfo.Name, network.CreateOptions{
							Driver:     networkInfo.Driver,
							Scope:      networkInfo.Scope,
							EnableIPv4: &networkInfo.EnableIPv4,
							EnableIPv6: &networkInfo.EnableIPv6,
							IPAM:       &networkInfo.IPAM,
							Internal:   networkInfo.Internal,
							Attachable: networkInfo.Attachable,
							Ingress:    networkInfo.Ingress,
							ConfigOnly: networkInfo.ConfigOnly,
							ConfigFrom: &networkInfo.ConfigFrom,
							Options:    networkInfo.Options,
							Labels:     networkInfo.Labels,
						})
						if err != nil {
							if function.ErrorHasKeyword(err, "Pool overlaps with other one on this address space") {
								self.JsonResponseWithError(http,
									function.ErrorMessage(
										".containerBackupRestoreNetworkConflict",
										"name", networkInfo.Name,
										"subnet", strings.Join(function.PluckArrayWalk(networkInfo.IPAM.Config, func(i network.IPAMConfig) (string, bool) {
											return i.Subnet, true
										}), ","),
									), 500)
								return
							}
							self.JsonResponseWithError(http, err, 500)
							return
						}
					}
				}
			}

			networkingConfig := &network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			}
			if containerInfo.NetworkSettings != nil && !function.IsEmptyMap(containerInfo.NetworkSettings.Networks) {
				for name, settings := range containerInfo.NetworkSettings.Networks {
					if name == network.NetworkBridge {
						continue
					}
					settings.EndpointID = ""
					settings.NetworkID = ""
					networkingConfig.EndpointsConfig[name] = settings
				}
			}
			compatContainerInfo, err := docker.Sdk.ContainerInspectCompat(containerInfo)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			_, err = docker.Sdk.Client.ContainerCreate(docker.Sdk.Ctx, compatContainerInfo.Config, compatContainerInfo.HostConfig, networkingConfig, &v1.Platform{}, newContainerName)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, newContainerName, container.StartOptions{})
			if err != nil {
				for _, inspect := range networkCreate {
					_ = docker.Sdk.Client.NetworkRemove(docker.Sdk.Ctx, inspect.Name)
				}
				_ = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, newContainerName, container.RemoveOptions{})
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}

		for _, volume := range item.Volume {
			destPath := "/"
			for _, mount := range containerInfo.Mounts {
				if strings.HasSuffix(function.GetSha256([]byte(mount.Destination)), filepath.Base(volume)) {
					// 导出的数据是按最后一个目录或是文件名存放，所以需要脱一级目录做为根目录
					destPath = filepath.Dir(mount.Destination)
				}
			}
			reader, _ := b.Reader.ReadBlobs(volume)
			gzReader, _ := gzip.NewReader(reader)
			tarReader := tar.NewReader(gzReader)
			if options, err := imports.NewFileImport(destPath, imports.WithImportTar(tarReader)); err == nil {
				err = docker.Sdk.ContainerImport(docker.Sdk.Ctx, newContainerName, options)
				if err != nil {
					slog.Warn("container backup restore", "error", err)
				}
			}
			_ = gzReader.Close()
		}
	}
	self.JsonSuccessResponse(http)
	return
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

func (self ContainerBackup) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error
	backupRow, err := dao.Backup.Where(dao.Backup.ID.Eq(params.Id)).First()
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
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
	type backupInfo struct {
		container.InspectResponse
		EnableImage bool
	}
	containerInfoList := make([]backupInfo, 0)
	for _, item := range manifest {
		config, err := b.Reader.ReadBlobsContent(item.Config)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		containerInfo := backupInfo{}
		err = json.Unmarshal(config, &containerInfo)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if item.Image != "" {
			containerInfo.EnableImage = true
		}
		containerInfoList = append(containerInfoList, containerInfo)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"detail": containerInfoList,
	})
	return
}
