package compose

const (
	ContainerDefaultName = "%CONTAINER_DEFAULT_NAME%"
)

var placeholderList = map[string]string{
	ContainerDefaultName: "",
}

func ReplacePlaceholder(str string) string {
	if value, ok := placeholderList[str]; ok {
		return value
	} else {
		return str
	}
}
