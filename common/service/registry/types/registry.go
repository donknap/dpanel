package types

// Credential keeps the access key and/or secret for the related registry
type Credential struct {
	// Type of the credential
	Type string `json:"type"`
	// The key of the access account, for OAuth token, it can be empty
	AccessKey string `json:"access_key"`
	// The secret or password for the key
	AccessSecret string `json:"access_secret"`
}
