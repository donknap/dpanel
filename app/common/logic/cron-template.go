package logic

import "github.com/donknap/dpanel/common/accessor"

type CronTemplateItem struct {
	Name        string             `json:"name"`
	Environment []accessor.EnvItem `json:"environment"`
	Script      string             `json:"script"`
}

type CronTemplate struct {
}

func (self CronTemplate) Template() {

}
