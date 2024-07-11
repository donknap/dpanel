package accessor

type BackupSettingOption struct {
	BackupTargetType string `json:"backupTargetType"`
	BackupTar        string `json:"backupTar"`
	BackupPath       string `json:"backupPath"`
}
