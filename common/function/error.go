package function

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

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
	return errors.New(string(result))
}
