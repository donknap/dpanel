package controller

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
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
	"github.com/donknap/dpanel/common/service/plugin"
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
		Id                         string   `json:"id" binding:"required"`
		EnableBackupImage          bool     `json:"enableBackupImage"`
		EnableBackupImageContainer bool     `json:"enableBackupImageContainer"`
		EnableBackupVolume         bool     `json:"enableBackupVolume"`
		BackupVolumeList           []string `json:"backupVolumeList"`
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
			BackupTargetType: define.DockerContainerBackupTypeSnapshot,
			BackupTar:        filepath.ToSlash(backupRelTar),
			VolumePathList:   make([]string, 0),
			Status:           define.DockerImageBuildStatusProcess,
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
			_ = notice.Message{}.Info(".containerBackupFinish", "name", strings.TrimLeft(containerInfo.Name, "/"))
			if closeErr := b.Close(); closeErr != nil {
				backupRow.Setting.Status = define.DockerImageBuildStatusError
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
	createErr := func() error {
		var err error
		item := backup.Manifest{}
		if params.EnableBackupImage {
			imageId := containerInfo.Image
			imageName := containerInfo.Config.Image
			// 提交容器为新镜像
			if params.EnableBackupImageContainer {
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

		if params.EnableBackupVolume {
			if !function.IsEmptyArray(containerInfo.Mounts) {
				for _, mount := range containerInfo.Mounts {
					if !function.IsEmptyArray(params.BackupVolumeList) && !function.InArray(params.BackupVolumeList, mount.Destination) {
						continue
					}
					stat, err := docker.Sdk.Client.ContainerStatPath(progress.Context(), params.Id, mount.Destination)
					if err != nil {
						return err
					}

					out, info, err := docker.Sdk.Client.CopyFromContainer(progress.Context(), params.Id, mount.Destination)
					if err != nil {
						return err
					}

					if info.Size > 0 {
						savePath, err := b.Writer.WriteBlobReader(function.Sha256([]byte(mount.Destination)), out)
						if err != nil {
							return err
						}
						item.VolumeList = append(item.VolumeList, backup.ManifestVolumeInfo{
							SavePath:    savePath,
							Destination: mount.Destination,
							Source:      mount.Source,
							Mode:        stat.Mode,
						})
						item.Volume = append(item.Volume, savePath)
					}

					backupRow.Setting.VolumePathList = append(backupRow.Setting.VolumePathList, mount.Destination)
				}
				sort.Slice(item.VolumeList, func(i, j int) bool {
					if item.VolumeList[i].Mode.IsDir() != item.VolumeList[j].Mode.IsDir() {
						return !item.VolumeList[i].Mode.IsDir()
					}
					return item.VolumeList[i].Source < item.VolumeList[j].Source
				})
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

	backupRow.Setting.Status = define.DockerImageBuildStatusError
	if createErr != nil {
		backupRow.Setting.Error = createErr.Error()
	} else {
		err = b.Writer.WriteConfigFile("manifest.json", manifest)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		backupRow.Setting.Status = define.DockerImageBuildStatusSuccess
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
	if createErr != nil {
		self.JsonResponseWithError(http, createErr, 500)
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
		if err != nil || containerInfo.ContainerJSONBase == nil {
			slog.Warn("container backup restore parse container", "json", string(config), "error", err)
			self.JsonResponseWithError(http, errors.Join(errors.New("failed to parse container configuration"), err), 500)
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

		runContainer := false
		var networkCreate []network.Inspect

		newContainerName := containerInfo.Name
		if _, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, newContainerName); err != nil {
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
						if networkInfo.IPAM.Config != nil {
							for i, ipamConfig := range networkInfo.IPAM.Config {
								// fix docker 导出 ipv6 网关地址的时候附带了 /64
								//"IPAM": {
								//    "Config": [
								//        {
								//            "Gateway": "172.18.0.1",
								//            "Subnet": "172.18.0.0/16"
								//        },
								//        {
								//            "Gateway": "fd86:9ba5:b9cc::1/64",
								//            "Subnet": "fd86:9ba5:b9cc::/64"
								//        }
								//    ],
								//    "Driver": "default",
								//    "Options": {}
								//}
								if b, _, ok := strings.Cut(ipamConfig.Gateway, "/"); ok {
									networkInfo.IPAM.Config[i].Gateway = b
								}
							}
						}
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

					if settings.IPAMConfig != nil {
						settings.IPAMConfig.IPv6Address = ""
						settings.IPAMConfig.IPv4Address = ""
					}

					settings.Gateway = ""
					settings.IPAddress = "" // 这里把 Ip 置空，直接采用网络的子网自动分配，否则可能会造成 ip 与网络不

					networkingConfig.EndpointsConfig[name] = settings
				}
			}

			compactContainerInfo, err := docker.Sdk.ContainerInspectCompact(containerInfo)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			_, err = docker.Sdk.Client.ContainerCreate(docker.Sdk.Ctx, compactContainerInfo.Config, compactContainerInfo.HostConfig, networkingConfig, &v1.Platform{}, newContainerName)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			// 创建完成后不能启动，要等恢复完数据后才可以
			runContainer = true
		}

		// 兼容旧的数据
		if !function.IsEmptyArray(item.Volume) && function.IsEmptyArray(item.VolumeList) {
			item.VolumeList = function.PluckArrayWalk(item.Volume, func(volume string) (backup.ManifestVolumeInfo, bool) {
				mount, _, ok := function.PluckArrayItemWalk(containerInfo.Mounts, func(item container.MountPoint) bool {
					return strings.HasSuffix(function.Sha256([]byte(item.Destination)), path.Base(volume))
				})
				if !ok {
					return backup.ManifestVolumeInfo{}, false
				}
				return backup.ManifestVolumeInfo{
					Destination: mount.Destination,
					Source:      mount.Source,
					SavePath:    volume,
					Mode:        os.ModeDir,
				}, true
			})
		}

		if !function.IsEmptyArray(item.VolumeList) {
			err = func() error {
				var proxyContainerName string

				// 仅当有挂载文件的时候才新建文件管理助手
				if _, _, ok := function.PluckArrayItemWalk(item.VolumeList, func(item backup.ManifestVolumeInfo) bool {
					return item.Mode.IsRegular()
				}); ok {
					ctx, ctxCancel := context.WithCancel(docker.Sdk.Ctx)
					defer ctxCancel()
					proxyContainerName, err = plugin.NewHostExplorer(ctx, docker.Sdk)
					if err != nil {
						return err
					}
				}

				for _, volume := range item.VolumeList {
					targetImportContainerName := newContainerName
					targetImportPath := "/"

					reader, _ := b.Reader.ReadBlobs(volume.SavePath)
					gzReader, _ := gzip.NewReader(reader)
					tarReader := tar.NewReader(gzReader)

					// 因为从 docker 导出目录的时候，不会存储一级目录，而是从二级目录开始。
					// 例如 docker cp caddy:/etc/caddy/ . 只会保存 caddy 目录，那么这里恢复的时候，也需要脱去一层目录
					importOption := make([]imports.ImportFileOption, 0)
					if volume.Mode.IsRegular() {
						targetImportPath = path.Join("/", "mnt", "host", path.Dir(volume.Source))
						targetImportContainerName = proxyContainerName
						importOption = append(importOption, imports.WithImportFileInTar(tarReader, path.Base(volume.Source), func(header *tar.Header) bool {
							return strings.HasSuffix(volume.Destination, header.Name)
						}))
						_, err = docker.Sdk.ContainerExecResult(docker.Sdk.Ctx, proxyContainerName, "mkdir -p "+targetImportPath)
						if err != nil {
							return err
						}
					} else {
						targetImportPath = path.Dir(volume.Destination)
						targetImportContainerName = newContainerName
						importOption = append(importOption, imports.WithImportTar(tarReader))
					}

					if importFiles, err := imports.NewFileImport("/", importOption...); err == nil {
						err = docker.Sdk.ContainerImport(docker.Sdk.Ctx, targetImportContainerName, targetImportPath, importFiles.Reader())
						importFiles.Close()
						if err != nil {
							return err
						}
					}
					_ = gzReader.Close()
				}
				return nil
			}()

			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}

		if runContainer {
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
	}
	self.JsonSuccessResponse(http)
	return
}

func (self ContainerBackup) GetList(http *gin.Context) {
	type ParamsValidate struct {
		ContainerId string `json:"containerId"`
		Page        int    `json:"page" binding:"omitempty,gt=0"`
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
