package event

const (
	SettingSaveEvent = "setting_save"
)

type SettingPayload struct {
	GroupName string
	Name      string
}
