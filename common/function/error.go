package function

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type messageError struct {
	value string
}

func (e *messageError) Error() string {
	return e.value
}

func IsErrorMessage(err error) bool {
	var target *messageError
	return errors.As(err, &target)
}

func ErrorHasKeyword(e error, keyword ...string) bool {
	for _, k := range keyword {
		if strings.Contains(e.Error(), k) {
			return true
		}
	}
	return false
}

func ErrorMessage(title string, message ...string) error {
	jsonMessage, _ := json.Marshal(message)
	row := &gin.H{
		"title":     title,
		"message":   string(jsonMessage),
		"type":      "error",
		"createdAt": time.Now().Local(),
	}
	result, _ := json.Marshal(row)
	return &messageError{value: string(result)}
}
