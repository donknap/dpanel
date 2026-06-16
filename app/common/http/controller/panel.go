package controller

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/backup"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/mcuadros/go-version"
	"github.com/mholt/archives"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/we7coreteam/registry-go-sdk"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Panel struct {
	controller.Abstract
}

func (self Panel) Usage(http *gin.Context) {
	type pathUsageItem struct {
		*types.ValueItem
		Used        int64   `json:"used"`        // 路径已用空间 (Bytes)
		UsedSize    string  `json:"usedSize"`    // 路径已使用大小
		UsedPercent float64 `json:"usedPercent"` // 路径使用率 (%)
	}

	var diskTotal uint64
	var panelTotal uint64

	if v, err := disk.Usage(storage.Local{}.GetSaveRootPath()); err == nil {
		diskTotal = v.Total
	}

	if v, err := function.PathSize(storage.Local{}.GetStorageLocalPath()); err == nil {
		panelTotal = uint64(v)
	}

	savePath := function.PluckArrayWalk((logic.Panel{}).GetPanelPath(), func(item *types.ValueItem) (*pathUsageItem, bool) {
		usageItem := &pathUsageItem{
			ValueItem:   item,
			Used:        0,
			UsedSize:    "",
			UsedPercent: 0,
		}
		realPath := filepath.Join(storage.Local{}.GetStorageLocalPath(), filepath.Clean(item.Value))
		if _, err := os.Stat(realPath); errors.Is(err, os.ErrNotExist) {
			return nil, false
		}
		if v, err := function.PathSize(realPath); err == nil {
			usageItem.Used = v
		} else {
			usageItem.Used = 0
		}
		usageItem.UsedSize = units.HumanSize(float64(usageItem.Used))
		usageItem.UsedPercent = float64(usageItem.Used) / float64(diskTotal) * 100
		return usageItem, true
	})

	sort.Slice(savePath, func(i, j int) bool {
		return savePath[i].Used > savePath[j].Used
	})

	self.JsonResponseWithoutError(http, gin.H{
		"pathUsage":  savePath,
		"diskUsage":  diskTotal,
		"panelUsage": panelTotal,
	})
	return
}

func (self Panel) Backup(http *gin.Context) {
	type ParamsValidate struct {
		BackupVolumePathList   []string `json:"backupPathList"`
		EnableBackupVolume     bool     `json:"enableBackupVolume"`
		EnableBackupApp        bool     `json:"enableBackupApp"`
		IgnoreVolumePathPrefix []string `json:"ignoreVolumePathPrefix"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	params.IgnoreVolumePathPrefix = append(params.IgnoreVolumePathPrefix, "backup/dpanel/dpanel-main", "storage/temp")

	panelAllPath := function.PluckArrayWalk((logic.Panel{}).GetPanelPath(), func(item *types.ValueItem) (string, bool) {
		return item.Value, true
	})

	if function.IsEmptyArray(params.BackupVolumePathList) {
		params.BackupVolumePathList = panelAllPath
	}

	backupTime := time.Now().Format(define.DateYmdHis)
	suffix := fmt.Sprintf("dpanel-main-%s", backupTime)
	backupRelTar := filepath.Join("dpanel", suffix+".snapshot")
	backupTar := filepath.Join(storage.Local{}.GetBackupPath(), backupRelTar)

	b, err := backup.New(
		backup.WithTarPathPrefix("dpanel"),
		backup.WithPath(backupTar),
		backup.WithWriter(),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = b.Close()
	}()

	info := backup.Info{
		Extend: gin.H{
			"Version": facade.GetConfig().Get("app.version"),
			"Family":  facade.GetConfig().Get("app.family"),
		},
		Backup: &entity.Backup{
			ID:          0,
			ContainerID: "",
			Setting: &accessor.BackupSettingOption{
				BackupTargetType: define.DockerContainerBackupTypeSnapshot,
				BackupTar:        filepath.ToSlash(backupRelTar),
				VolumePathList:   make([]string, 0),
				Status:           define.DockerImageBuildStatusSuccess,
			},
			CreatedAt: time.Now(),
		},
	}
	info.Docker, err = docker.Sdk.Client.ServerVersion(b.Context())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	params.BackupVolumePathList = append(params.BackupVolumePathList, "dpanel.lic")
	manifest := make([]backup.Manifest, 0)
	targetFile, err := archives.FilesFromDisk(b.Context(), nil, function.PluckArrayMapWalk(params.BackupVolumePathList, func(item string) (string, string, bool) {
		if !function.InArray(panelAllPath, item) {
			return "", "", false
		}
		return filepath.Join(storage.Local{}.GetStorageLocalPath(), item), item, true
	}))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	hash := make([]string, 0)
	targetFile = function.PluckArrayWalk(targetFile, func(item archives.FileInfo) (archives.FileInfo, bool) {
		if ok := function.InArrayWalk(params.IgnoreVolumePathPrefix, func(p string) bool {
			return strings.HasPrefix(item.NameInArchive, p)
		}); ok {
			return item, false
		}
		hash = append(hash, item.NameInArchive)
		return item, true
	})

	volumePath, err := b.Writer.WriteBlobFiles(function.Sha256Struct(hash), targetFile)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	manifest = append(manifest, backup.Manifest{
		Volume: []string{
			volumePath,
		},
	})
	err = b.Writer.WriteConfigFile("manifest.json", manifest)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = b.Writer.WriteConfigFile("info.json", info)

	self.JsonResponseWithoutError(http, gin.H{
		"path": backupTar,
	})
	return
}

func (self Panel) Proxy(http *gin.Context) {
	type ParamsValidate struct {
		Proxy   string `json:"proxy"`
		NoProxy string `json:"noProxy"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if params.Proxy != "" {
		if err := (logic.Panel{}).ValidateProxy(params.Proxy); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	dpanelInfo := logic.Setting{}.GetDPanelInfo()
	dpanelInfo.Proxy = params.Proxy
	dpanelInfo.NoProxy = params.NoProxy
	err := logic.Setting{}.Save(&entity.Setting{
		GroupName: logic.SettingGroupSetting,
		Name:      logic.SettingGroupSettingDPanelInfo,
		Value: &accessor.SettingValueOption{
			DPanelInfo: &dpanelInfo,
		},
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Proxy != "" {
		_ = os.Setenv("HTTP_PROXY", params.Proxy)
		_ = os.Setenv("HTTPS_PROXY", params.Proxy)
	} else {
		_ = os.Unsetenv("HTTP_PROXY")
		_ = os.Unsetenv("HTTPS_PROXY")
	}

	if params.NoProxy != "" {
		_ = os.Setenv("NO_PROXY", params.NoProxy)
	} else {
		_ = os.Unsetenv("NO_PROXY")
	}

	self.JsonSuccessResponse(http)
}

func (self Panel) Update(http *gin.Context) {
	type ParamsValidate struct {
		Name                    string         `json:"name"`
		InstallerDownloadSource string         `json:"installerDownloadSource"`
		DryRun                  bool           `json:"dryRun"`
		Params                  map[string]any `json:"params"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	dpanelInfo := logic.Setting{}.GetDPanelInfo()
	params.Name = dpanelInfo.Name
	if params.Name == "" {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonNotFoundDPanel), 500)
		return
	}

	if function.IsRunInDocker() {
		// 如果容器运行但是无法获取面板容器信息则直接报错
		if dpanelInfo.ContainerInfo.ContainerJSONBase == nil {
			if params.DryRun {
				self.JsonResponseWithoutError(http, gin.H{
					"command": "curl -sSL https://dpanel.cc/quick.sh | bash",
				})
				return
			}
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonNotFoundDPanel), 500)
			return
		}
		params.InstallerDownloadSource = "dpanel/installer:latest"
		for _, address := range []string{"registry.cn-hangzhou.aliyuncs.com", "docker.io"} {
			reg := registry.New(
				registry.WithAddress(address),
			)
			if err := reg.Client().Ping(); err != nil {
				slog.Debug("panel upgrade select installer registry", "address", address, "error", err)
				continue
			}
			params.InstallerDownloadSource = address + "/dpanel/installer:latest"
			break
		}

	}
	cmd, err := logic.Panel{}.MakeUpdateCommand(function.StructToMap(params))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.DryRun {
		self.JsonResponseWithoutError(http, gin.H{
			"command": cmd,
		})
		return
	}

	commandExecutor, err := local.New(
		local.WithCommandName("/bin/sh"),
		local.WithArgs("-c", cmd),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	commandExecutor.WorkDir(storage.Local{}.GetStorageLocalPath())
	commandOutput, err := commandExecutor.RunInPip()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = commandOutput.Close()
		slog.Debug("panel upgrade output closed", "name", params.Name, "type", dpanelInfo.RunIn)
	}()
	scanner := bufio.NewScanner(commandOutput)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		slog.Debug("panel upgrade output", "name", params.Name, "type", dpanelInfo.RunIn, "line", line)
	}
	if err := scanner.Err(); err != nil {
		slog.Debug("panel upgrade output error", "name", params.Name, "type", dpanelInfo.RunIn, "error", err)
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Panel) BackupList(http *gin.Context) {
	root := logic.Panel{}.SaveRootPath()
	type backupFileItem struct {
		Path      string    `json:"path"`
		CreatedAt time.Time `json:"createdAt"`
		Size      int64     `json:"size"`
	}
	backupList := make([]backupFileItem, 0)
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if path == root || !strings.HasSuffix(path, ".snapshot") {
			return nil
		}
		if info.IsDir() {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		backupList = append(backupList, backupFileItem{
			Path:      rel,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	sort.Slice(backupList, func(i, j int) bool {
		return backupList[i].CreatedAt.After(backupList[j].CreatedAt)
	})

	self.JsonResponseWithoutError(http, gin.H{
		"list": backupList,
	})
	return
}

func (self Panel) BackupDelete(http *gin.Context) {
	type ParamsValidate struct {
		Name []string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	for _, s := range params.Name {
		backupFilePath := function.SafePathJoin(logic.Panel{}.SaveRootPath(), s)
		if _, err := os.Stat(backupFilePath); errors.Is(err, os.ErrNotExist) {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		err := function.SafeDelete(
			logic.Panel{}.SaveRootPath(),
			s,
		)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Panel) BackupDownload(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	backupFilePath := filepath.Join(logic.Panel{}.SaveRootPath(), function.SafeFileName(params.Name))
	if _, err := os.Stat(backupFilePath); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	downloadUrl, err := logic.Attach{}.PreDownload(backupFilePath, time.Second*10)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"downloadUrl": downloadUrl,
	})
	return
}

func (self Panel) BackupRestore(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	backupTar := filepath.Join(logic.Panel{}.SaveRootPath(), function.SafeFileName(params.Name))
	if _, err := os.Stat(backupTar); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	b, err := backup.New(
		backup.WithTarPathPrefix("dpanel"),
		backup.WithPath(backupTar),
		backup.WithReader(),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		b.Close()
	}()
	manifest, err := b.Reader.Manifest()
	if err != nil || function.IsEmptyArray(manifest) || len(manifest) == 0 {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}

	err = b.Reader.Extract(manifest[0].Volume[0], storage.Local{}.GetStorageLocalPath())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Panel) BackupImport(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	realTarFilePath := storage.Local{}.GetSaveRealPath(params.Name)
	if _, err := os.Stat(realTarFilePath); err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerBackupImportFileFailed), 500)
		return
	}
	defer func() {
		_ = os.Remove(realTarFilePath)
	}()

	b, err := backup.New(
		backup.WithPath(realTarFilePath),
		backup.WithReader(),
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	info, err := b.Reader.Info()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_ = b.Close()
	if info.Backup == nil || info.Backup.Setting == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerBackupImportFileFailed), 500)
		return
	}
	backupFileName := function.SafeFileName(info.Backup.Setting.BackupTar)
	if !strings.HasSuffix(backupFileName, ".snapshot") {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerBackupImportFileFailed), 500)
		return
	}
	backupRelTar := filepath.ToSlash(filepath.Join("dpanel", backupFileName))
	backupTarFile := function.SafePathJoin(storage.Local{}.GetBackupPath(), backupRelTar)
	_ = os.MkdirAll(filepath.Dir(backupTarFile), os.ModePerm)
	err = os.Rename(realTarFilePath, backupTarFile)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	info.Backup.ID = 0
	info.Backup.Setting.BackupTar = backupRelTar
	_ = dao.Backup.Save(info.Backup)
	self.JsonSuccessResponse(http)
	return
}

func (self Panel) CheckNewVersion(http *gin.Context) {
	currentVersion := facade.GetConfig().GetString("app.version")
	newVersion := ""
	reference := "latest"
	if strings.Index(currentVersion, ".") == 8 {
		reference = "beta"
	}
	option := make([]registry.Option, 0)
	option = append(option, registry.WithAddress(registry.RegistryDefaultHost, "registry.cn-hangzhou.aliyuncs.com"))
	reg := registry.New(option...)
	if manifest, _, err := reg.Client().PullManifest("dpanel/dpanel", reference); err == nil {
		for _, descriptor := range manifest.References() {
			if ver, ok := descriptor.Annotations[define.PanelLabelVersion]; ok {
				if version.Compare(ver, currentVersion, ">") {
					newVersion = ver
				}
				break
			}
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"version":    currentVersion,
		"newVersion": newVersion,
	})
	return
}
