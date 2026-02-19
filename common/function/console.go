package function

import "fmt"

const (
	ConsoleColorReset = "\033[0m"
	ConsoleColorGreen = "\033[32m"
	ConsoleColorRed   = "\033[31m"
)

func ConsoleWrite(color string, message string) string {
	return fmt.Sprintf("%s %s%s \n", color, message, ConsoleColorReset)
}

func ConsoleWriteError(message string) string {
	return ConsoleWrite(ConsoleColorRed, message)
}
