package logic

import (
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
)

// 全局配置
var (
	SettingGroupSetting                     = "setting"
	SettingGroupSettingServer               = "server" // 服务器
	SettingGroupSettingDocker               = "docker" // docker env
	SettingGroupSettingTwoFa                = "twoFa"  // 双因素
	SettingGroupSettingDiskUsage            = "diskUsage"
	SettingGroupSettingCheckContainerIgnore = "checkContainerIgnore"
	SettingGroupSettingDPanelInfo           = "DPanelInfo"
)

// 用户相关数据
var (
	SettingGroupUser        = "user"
	SettingGroupUserFounder = "founder"
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
	if setting == nil {
		return nil, errors.New("配置不存在")
	}
	return setting, nil

}

func (self Setting) GetValueById(id int32) (*entity.Setting, error) {
	setting, _ := dao.Setting.Where(dao.Setting.ID.Eq(id)).First()
	if setting == nil {
		return nil, errors.New("配置不存在")
	}
	return setting, nil
}

func (self Setting) GetDockerClient(name string) (*docker.Client, error) {
	if setting, err := self.GetValue(SettingGroupSetting, SettingGroupSettingDocker); err == nil {
		if item, ok := setting.Value.Docker[name]; ok {
			return item, nil
		}
	}
	return nil, errors.New("docker client not found " + name)
}

func (self Setting) GetDPanelInfo() (types.ContainerJSON, error) {
	if setting, err := self.GetValue(SettingGroupSetting, SettingGroupSettingDPanelInfo); err == nil && setting != nil && setting.Value != nil {
		return setting.Value.DPanelInfo, nil
	}
	return types.ContainerJSON{}, errors.New("dpanel container not found")
}

func (self Setting) Delete(groupName string, name string) error {
	_, _ = dao.Setting.Where(
		dao.Setting.GroupName.Eq(groupName),
		dao.Setting.Name.Eq(name),
	).Delete()
	return nil
}
