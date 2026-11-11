package store

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	sourcesync "github.com/donknap/dpanel/common/service/sync"
	"github.com/donknap/dpanel/common/types/define"
)

// 目录切换与应用索引发布必须顺序完成。
var storeSourceMutex sync.Mutex

var builtinStoreMirrorMap = map[string][]string{
	"https://github.com/qyg2297248353/appstore-dpanel": {
		"registry.cn-hangzhou.aliyuncs.com/dpanel/store:qyg2297248353",
		"dpanel/store:qyg2297248353",
	},
	"https://github.com/1Panel-dev/appstore": {
		"registry.cn-hangzhou.aliyuncs.com/dpanel/store:1panel",
		"dpanel/store:1panel",
	},
	"https://github.com/okxlin/appstore": {
		"registry.cn-hangzhou.aliyuncs.com/dpanel/store:okxlin",
		"dpanel/store:okxlin",
	},
	"https://github.com/DPanel-dev/plugin": {
		"registry.cn-hangzhou.aliyuncs.com/dpanel/plugin:latest",
		"dpanel/plugin:latest",
	},
}

type source struct {
	row          *entity.Store
	path         string
	tempPath     string
	dockerClient *docker.Client
}

// tempPath 由调用方指定，并须与商店目录位于同一文件系统，以支持目录切换。
func New(storeRow *entity.Store, tempPath string, optionList ...SourceOption) (StoreSource, error) {
	if storeRow == nil || storeRow.Setting == nil || !function.IsSafeString(storeRow.Name, function.StringRuleName) {
		return nil, os.ErrPermission
	}
	if tempPath == "" {
		return nil, fmt.Errorf("temporary path is required")
	}
	base := source{row: storeRow, path: function.SafePathJoin(storage.Local{}.GetStorePath(), storeRow.Name), tempPath: tempPath}
	for _, option := range optionList {
		base.dockerClient = option.DockerClient
	}
	repositoryURL := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(storeRow.Setting.Url), "/"), ".git")
	if mirrorList, ok := builtinStoreMirrorMap[repositoryURL]; ok {
		storeRow.Setting.Url = repositoryURL + ".git"
		// nil 使用内置来源，显式传入的地址或空列表由调用方决定。
		if storeRow.Setting.Mirror == nil {
			storeRow.Setting.Mirror = slices.Clone(mirrorList)
		}
	}
	switch storeRow.Setting.Type {
	case define.StoreTypeOnePanel, define.StoreTypeOnePanelLocal:
		return OnePanel{source: base}, nil
	case define.StoreTypeBaoTa:
		return BaoTa{source: base}, nil
	case define.StoreTypeCasaOs:
		return CasaOs{source: base}, nil
	case define.StoreTypePortainer:
		return Portainer{source: base}, nil
	case define.StoreTypeDPanel:
		return DPanel{source: base}, nil
	default:
		return nil, fmt.Errorf("unsupported store type %q", storeRow.Setting.Type)
	}
}

// Check 返回远端内容标识，不同步文件或更新本地版本记录。
func (self source) Check(ctx context.Context) (string, error) {
	if self.row.Setting.Type != define.StoreTypeDPanel && self.row.Setting.Type != define.StoreTypeOnePanel {
		return "", ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if len(self.row.Setting.Mirror) > 0 {
		var err error
		for _, address := range self.row.Setting.Mirror {
			if err = ctx.Err(); err != nil {
				return "", err
			}
			if sourcesync.IsGit(address) || !sourcesync.IsOci(address) {
				err = fmt.Errorf("store update mirror must be an OCI image")
				continue
			}
			var release string
			release, err = sourcesync.Release(ctx, address, "")
			if err != nil {
				continue
			}
			if release == "" {
				err = fmt.Errorf("store image has no release annotation")
				continue
			}
			return release, nil
		}
		return "", err
	}
	if !sourcesync.IsGit(self.row.Setting.Url) {
		return "", ErrUnsupported
	}
	gitURL, branch, _ := strings.Cut(self.row.Setting.Url, "#")
	refList := []string{"HEAD"}
	if branch != "" {
		refList = []string{"refs/heads/" + branch, "refs/tags/" + branch + "^{}", "refs/tags/" + branch}
	}
	cmd, err := local.New(
		local.WithCommandName("git"),
		local.WithArgs(append([]string{"ls-remote", "--exit-code", "--", gitURL}, refList...)...),
		local.WithEnv(append(os.Environ(), "GIT_TERMINAL_PROMPT=0")),
		local.WithCtx(ctx),
	)
	if err != nil {
		return "", err
	}
	defer cmd.Close()
	content, err := cmd.RunWithResult()
	if err != nil {
		return "", err
	}
	for _, ref := range refList {
		for line := range strings.SplitSeq(string(content), "\n") {
			fieldList := strings.Fields(line)
			if len(fieldList) != 2 || fieldList[1] != ref {
				continue
			}
			if _, err = hex.DecodeString(fieldList[0]); err != nil || (len(fieldList[0]) != 40 && len(fieldList[0]) != 64) {
				return "", fmt.Errorf("store Git source returned an invalid commit")
			}
			return fieldList[0], nil
		}
	}
	return "", fmt.Errorf("store Git source has no matching commit")
}

func (self source) saveList(appList []accessor.StoreAppDetail) error {
	if self.row.ID == 0 {
		return nil
	}
	if len(appList) > 0 {
		self.row.Setting.Apps = appList
	}
	self.row.Setting.Total = len(self.row.Setting.Apps)
	self.row.Setting.UpdatedAt = time.Now().Unix()
	return dao.Store.Save(self.row)
}

// GetApp 统一处理版本引用和来源默认值，也兼容尚未重新同步的既有应用索引。
func GetApp(storeRow *entity.Store, detail accessor.StoreAppInstallDetail) (accessor.StoreAppDetail, accessor.StoreAppVersionItem, error) {
	var app accessor.StoreAppDetail
	var version accessor.StoreAppVersionItem
	if storeRow == nil || storeRow.Setting == nil || storeRow.ID != detail.StoreId || !function.IsSafeString(storeRow.Name, function.StringRuleName) || !function.IsSafeString(detail.AppName, function.StringRuleName) {
		return app, version, os.ErrPermission
	}
	app, _, ok := function.PluckArrayItemWalk(storeRow.Setting.Apps, func(item accessor.StoreAppDetail) bool {
		return item.Name == detail.AppName
	})
	if !ok {
		return app, version, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	version, ok = app.Version[detail.Version]
	if !ok {
		return app, version, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	for count := 0; version.Ref != ""; count++ {
		if count >= len(app.Version) {
			return app, version, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
		}
		version, ok = app.Version[version.Ref]
		if !ok {
			return app, version, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
		}
	}
	relPath, err := filepath.Rel(storeRow.Name, function.SlashPathToSystem(version.ComposeFile))
	if err != nil || !filepath.IsLocal(relPath) || relPath == "." {
		return app, version, os.ErrPermission
	}
	switch storeRow.Setting.Type {
	case define.StoreTypeOnePanel, define.StoreTypeOnePanelLocal:
		version.Environment = onePanelEnvironment(app.Name, version.Environment)
	case define.StoreTypeBaoTa:
		version.Environment = baoTaEnvironment(version.Environment)
	}
	return app, version, nil
}

func (self source) PreInstall(detail accessor.StoreAppInstallDetail) error {
	_, version, err := GetApp(self.row, detail)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(storage.Local{}.GetStorePath())
	if err != nil {
		return err
	}
	defer root.Close()
	appRoot, err := root.OpenRoot(filepath.Dir(function.SlashPathToSystem(version.ComposeFile)))
	if err != nil {
		return err
	}
	defer appRoot.Close()
	info, err := appRoot.Stat(filepath.Base(version.ComposeFile))
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("store app %q version %q: empty or invalid compose YAML", detail.AppName, detail.Version)
	}
	return nil
}
