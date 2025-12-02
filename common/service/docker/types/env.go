package types

import (
	"fmt"
	"strings"

	"github.com/donknap/dpanel/common/function"
)

const (
	EnvValueRuleRequired = 1 << iota
	EnvValueRuleDisabled
	EnvValueRuleInEnvFile
	_
	_
	_
	_
	_
	_
	_
	EnvValueTypeNumber
	EnvValueTypeText
	EnvValueTypeSelect
	EnvValueTypeSelectMultiple
	EnvValueTypeOnePanel
	EnvValueTypeBaoTa
)

func NewEnvItemFromString(s string) EnvItem {
	if k, v, ok := strings.Cut(s, "="); ok {
		return EnvItem{
			Name:  k,
			Value: v,
		}
	} else {
		return EnvItem{
			Name:  s,
			Value: "",
		}
	}
}

func NewEnvItemFromKV(k, v string) EnvItem {
	return EnvItem{
		Name:  k,
		Value: v,
	}
}

func NewValueItemWithArray(s ...string) []ValueItem {
	return function.PluckArrayWalk(s, func(item string) (ValueItem, bool) {
		return ValueItem{
			Name:  item,
			Value: item,
		}, true
	})
}

type EnvItem struct {
	Label  string            `json:"label,omitempty" yaml:"label,omitempty"` // Deprecated: instead Labels["zh"]
	Labels map[string]string `json:"labels,omitempty"`
	Name   string            `json:"name"`
	Value  string            `json:"value"`
	Rule   *EnvValueRule     `json:"rule,omitempty"`
}

func (self EnvItem) String() string {
	return fmt.Sprintf("%s=%s", self.Name, self.Value)
}

type EnvValueRule struct {
	Kind   int         `json:"kind,omitempty" yaml:"kind,omitempty"`
	Option []ValueItem `json:"option,omitempty" yaml:"option,omitempty"`
}

func (self EnvValueRule) IsInEnvFile() bool {
	return self.Kind&EnvValueRuleInEnvFile != 0
}
