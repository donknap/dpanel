package controller

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Panel struct {
	controller.Abstract
}

func (self Panel) Usage(c *gin.Context) {
	type pathItem struct {
		Name        string  `json:"name"`
		Path        string  `json:"path"`
		Used        int64   `json:"used"`        // 路径已用空间 (Bytes)
		UsedSize    string  `json:"usedSize"`    // 路径已使用大小
		UsedPercent float64 `json:"usedPercent"` // 路径使用率 (%)
	}

	savePath := []*pathItem{
		{Name: "database", Path: "/dpanel.db"},
		{Name: "store", Path: "/store"},
		{Name: "backup", Path: "/backup"},
		{Name: "compose-local", Path: "/compose"},
	}

	if setting, err := (logic.Setting{}).GetValue(logic.SettingGroupSetting, logic.SettingGroupSettingDocker); err == nil {
		for _, item := range setting.Value.Docker {
			if item.EnableComposePath {
				name := fmt.Sprintf("compose-%s", item.Name)
				savePath = append(savePath, &pathItem{
					Name: name,
					Path: name,
				})
			}
		}
	}

	savePath = append(savePath,
		&pathItem{Name: "export/file", Path: "/storage/export/file"},
		&pathItem{Name: "export/container", Path: "/storage/export/container"},
		&pathItem{Name: "export/image", Path: "/storage/export/image"},
		&pathItem{Name: "export/panel", Path: "/storage/export/panel"},
		&pathItem{Name: "temp", Path: "/storage/temp"},
	)
	var diskTotal uint64
	var panelTotal uint64

	if v, err := disk.Usage(storage.Local{}.GetSaveRootPath()); err == nil {
		diskTotal = v.Total
	}
	if v, err := function.PathSize(storage.Local{}.GetStorageLocalPath()); err == nil {
		panelTotal = uint64(v)
	}

	for _, item := range savePath {
		if v, err := function.PathSize(filepath.Join(storage.Local{}.GetStorageLocalPath(), item.Path)); err == nil {
			item.Used = v
		} else {
			item.Used = 0
		}
		item.UsedSize = units.HumanSize(float64(item.Used))
		item.UsedPercent = float64(item.Used) / float64(diskTotal) * 100
	}

	self.JsonResponseWithoutError(c, gin.H{
		"pathUsage":  savePath,
		"diskUsage":  diskTotal,
		"panelUsage": panelTotal,
	})
	return
}

func (self Panel) Backup(http *gin.Context) {
	type ParamsValidate struct {
		IgnorePath []string `json:"ignorePath"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	// sock 目录忽略掉，无法打包
	params.IgnorePath = append(params.IgnorePath, "/sock")

	rootPath := storage.Local{}.GetStorageLocalPath()
	backupFile, err := storage.Local{}.CreateSaveFile(
		path.Join("export",
			"panel",
			fmt.Sprintf("dpanel_backup_%s.tar.gz", time.Now().Format(define.DateYmdHis))))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	backupFilePath := backupFile.Name()
	_ = backupFile.Close()

	ignoreMap := make(map[string]bool)
	for _, p := range params.IgnorePath {
		cleanPath := strings.TrimPrefix(filepath.Clean(p), "/")
		absIgnorePath := filepath.Join(rootPath, cleanPath)
		ignoreMap[absIgnorePath] = true
	}

	var targetPaths []string
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	for _, entry := range entries {
		fullPath := filepath.Join(rootPath, entry.Name())
		if !ignoreMap[fullPath] {
			targetPaths = append(targetPaths, fullPath)
		}
	}

	if len(targetPaths) == 0 {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}

	err = function.Tar(backupFilePath, targetPaths, "", true, func(path string, info os.FileInfo) bool {
		return ignoreMap[path]
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonResponseWithoutError(http, gin.H{
		"path": backupFilePath,
	})
	return
}
