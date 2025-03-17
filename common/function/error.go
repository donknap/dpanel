package function

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"strings"
	"time"
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
