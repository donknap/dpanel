package logic

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	types2 "github.com/donknap/dpanel/common/types"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

// 全局配置
var (
	SettingGroupSetting                     = "setting"
	SettingGroupSettingServer               = "server" // 服务器
	SettingGroupSettingDocker               = "docker" // docker env
	SettingGroupSettingTwoFa                = "twoFa"  // 双因素
	SettingGroupSettingDiskUsage            = "diskUsage"
	SettingGroupSettingCheckContainerIgnore = "containerCheckIgnoreUpgrade"
	SettingGroupSettingCheckContainerAll    = "containerCheckAllUpgrade"
	SettingGroupSettingDPanelInfo           = "DPanelInfo"
	SettingGroupSettingThemeConfig          = "themeConfig"
	SettingGroupSettingThemeUserConfig      = "themeUserConfig"
	SettingGroupSettingThemeConsoleConfig   = "themeConsoleConfig"
	SettingGroupSettingDnsApi               = "dnsApi"
	SettingGroupSettingTag                  = "tag"
	SettingGroupSettingLogin                = "login"
	SettingGroupSettingNotification         = "notification"
	SettingGroupSettingConsoleInstance      = "consoleInstance"
)

// 用户相关数据
var (
	SettingGroupUser        = "user"
	SettingGroupUserFounder = "founder"
	SettingGroupUserManager = "manager"
	SettingGroupUserMember  = "member"

	SettingGroupUserStatusEnable  = uint8(2)
	SettingGroupUserStatusDisable = uint8(1)
)

type Setting struct {
}

func (self Setting) Save(settingRow *entity.Setting) error {
	oldSetting, _ := dao.Setting.Where(
		dao.Setting.GroupName.Eq(settingRow.GroupName),
		dao.Setting.Name.Eq(settingRow.Name),
	).First()
	if oldSetting == nil {
		err := dao.Setting.Create(settingRow)
		if err != nil {
			return err
		}
	} else {
		_, err := dao.Setting.Where(
			dao.Setting.GroupName.Eq(settingRow.GroupName),
			dao.Setting.Name.Eq(settingRow.Name),
		).Updates(settingRow)
		if err != nil {
			return err
		}
	}
	return nil
}

func (self Setting) GetValue(groupName string, name string) (*entity.Setting, error) {
	setting, _ := dao.Setting.Where(
		dao.Setting.GroupName.Eq(groupName),
		dao.Setting.Name.Eq(name),
	).First()
	if setting == nil || setting.Value == nil {
		return nil, fmt.Errorf("%s:%s setting not found", groupName, name)
	}
	return setting, nil

}

func (self Setting) GetValueById(id int32) (*entity.Setting, error) {
	setting, _ := dao.Setting.Where(dao.Setting.ID.Eq(id)).First()
	if setting == nil {
		return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	return setting, nil
}

func (self Setting) GetDPanelInfo() types2.DPanelInfo {
	result := types2.DPanelInfo{}
	self.GetByKey(SettingGroupSetting, SettingGroupSettingDPanelInfo, &result)
	existingName := result.Name
	if !function.IsRunInDocker() {
		if executablePath, err := os.Executable(); err == nil {
			executableName := strings.TrimSpace(filepath.Base(executablePath))
			if executableName != "" && executableName != "." && executableName != string(filepath.Separator) {
				result.Name = executableName
			}
		}
	}
	if result.Name == "" {
		result.Name = existingName
	}
	if result.Name == "" {
		result.Name = strings.TrimSpace(facade.GetConfig().GetString("app.name"))
	}
	result.Version = facade.GetConfig().GetString("app.version")
	result.Family = facade.GetConfig().GetString("app.family")
	result.Env = facade.GetConfig().GetString("app.env")
	result.BaseURL = facade.GetConfig().GetString("system.baseurl")
	result.ServerHost = facade.GetConfig().GetString("server.http.host")
	result.ServerPort = facade.GetConfig().GetInt("server.http.port")
	result.LogConsoleLevel = facade.GetConfig().GetString("log.console.level")
	result.LogFileLevel = facade.GetConfig().GetString("log.file.level")
	result.StorageLocalPath = facade.GetConfig().GetString("system.storage.local.path")

	if result.ServerHost == "" {
		result.ServerHost = os.Getenv("APP_SERVER_HOST")
	}
	if result.ServerPort <= 0 {
		if v := os.Getenv("APP_SERVER_PORT"); v != "" {
			if port, err := strconv.Atoi(v); err == nil {
				result.ServerPort = port
			}
		}
	}
	if result.BaseURL == "" {
		result.BaseURL = os.Getenv("DP_SYSTEM_BASEURL")
	}
	if result.LogConsoleLevel == "" {
		result.LogConsoleLevel = os.Getenv("DP_LOG_CONSOLE_LEVEL")
	}
	if result.LogFileLevel == "" {
		result.LogFileLevel = os.Getenv("DP_LOG_FILE_LEVEL")
	}
	if result.StorageLocalPath == "" {
		result.StorageLocalPath = os.Getenv("STORAGE_LOCAL_PATH")
	}
	if result.Dns == "" {
		result.Dns = os.Getenv("DP_DNS")
	}
	if result.Proxy == "" {
		if v := os.Getenv("HTTP_PROXY"); v != "" {
			result.Proxy = v
		} else {
			result.Proxy = os.Getenv("HTTPS_PROXY")
		}
	}
	if result.NoProxy == "" {
		result.NoProxy = os.Getenv("NO_PROXY")
	}

	if len(result.Version) == len(define.DateShowVersion) && strings.Count(result.Version, ".") == 1 {
		if t, err := time.ParseInLocation(define.DateShowVersion, result.Version, time.UTC); err == nil {
			result.IsDev = true
			result.Version = t.Local().Format(define.DateShowVersion)
		}
	}

	result.IsCe = result.Family != "pe"
	result.IsLite = result.Env == "lite"
	return result
}

func (self Setting) Delete(groupName string, name string) error {
	_, _ = dao.Setting.Where(
		dao.Setting.GroupName.Eq(groupName),
		dao.Setting.Name.Eq(name),
	).Delete()
	return nil
}

func (self Setting) GetByKey(group, name string, value interface{}) (exists bool) {
	setting, err := self.GetValue(group, name)
	if err != nil {
		return false
	}
	if value != nil {
		switch v := value.(type) {
		case *map[string]*types.DockerEnv:
			if setting.Value.Docker != nil {
				exists = true
				*v = setting.Value.Docker
			}
		case *accessor.DiskUsage:
			if setting.Value.DiskUsage != nil {
				exists = true
				*v = *setting.Value.DiskUsage
			}
		case *accessor.TwoFa:
			if setting.Value.TwoFa != nil {
				exists = true
				*v = *setting.Value.TwoFa
			}
		case *accessor.ContainerCheckIgnoreUpgrade:
			if setting.Value.ContainerCheckIgnoreUpgrade != nil {
				exists = true
				*v = setting.Value.ContainerCheckIgnoreUpgrade
			}
		case *types2.DPanelInfo:
			if setting.Value.DPanelInfo != nil {
				exists = true
				*v = *setting.Value.DPanelInfo
			}
		case *accessor.ThemeConfig:
			if setting.Value.ThemeConfig != nil {
				exists = true
				*v = *setting.Value.ThemeConfig
			}
		case *accessor.ThemeConsoleConfig:
			if setting.Value.ThemeConfig != nil {
				exists = true
				*v = *setting.Value.ThemeConsoleConfig
			}
		case *accessor.ThemeUserConfig:
			if setting.Value.ThemeUserConfig != nil {
				exists = true
				*v = *setting.Value.ThemeUserConfig
			}
		case *[]accessor.DnsApi:
			if setting.Value.DnsApi != nil {
				exists = true
				*v = setting.Value.DnsApi
			}
		case *accessor.Notification:
			if setting.Value.Notification != nil {
				exists = true
				*v = *setting.Value.Notification
			}
		case *accessor.ContainerCheckAllUpgrade:
			if setting.Value.ContainerCheckAllUpgrade != nil {
				exists = true
				*v = *setting.Value.ContainerCheckAllUpgrade
			}
		case *[]accessor.Tag:
			if setting.Value.Tag != nil {
				exists = true
				*v = setting.Value.Tag
			}
		case *accessor.Login:
			if setting.Value.Login != nil {
				exists = true
				*v = *setting.Value.Login
			}
		case *accessor.ConsoleInstance:
			if setting.Value.ConsoleInstance != nil {
				exists = true
				*v = *setting.Value.ConsoleInstance
			}
		case *entity.Setting:
			*v = *setting
		}
	}
	return exists
}
