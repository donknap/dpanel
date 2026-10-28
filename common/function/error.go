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

func (e *messageError) Is(target error) bool {
	targetError, ok := target.(*messageError)
	if !ok || e == nil || targetError == nil {
		return false
	}
	var message, targetMessage struct {
		Title *string `json:"title"`
	}
	if err := json.Unmarshal([]byte(e.value), &message); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(targetError.value), &targetMessage); err != nil {
		return false
	}
	return message.Title != nil && targetMessage.Title != nil && *message.Title == *targetMessage.Title
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
