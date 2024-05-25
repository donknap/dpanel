package logic

type Image struct {
}

type ImageNameOption struct {
	Registry string
	Name     string
	Version  string
}

func (self Image) GetImageName(option *ImageNameOption) (imageName string) {
	imageName = option.Name
	if option.Name == "" {
		return imageName
	}
	if option.Registry != "" {
		imageName = option.Registry + "/" + option.Name
	}
	if option.Version == "" {
		imageName += ":latest"
	} else {
		imageName += ":" + option.Version
	}
	return imageName
}
