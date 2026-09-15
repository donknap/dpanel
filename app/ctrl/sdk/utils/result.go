package utils

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

type Result struct{}

var quiet atomic.Bool

func SetQuiet(value bool) {
	quiet.Store(value)
}

func IsQuiet() bool {
	return quiet.Load()
}

func (self Result) Error(err error) {
	str, err := json.Marshal(gin.H{
		"error": err.Error(),
		"code":  500,
	})
	if err != nil {
		self.Error(err)
		return
	}

	fmt.Println(string(str))
	return
}

func (self Result) Errorf(format string, a ...any) {
	self.Error(fmt.Errorf(format, a))
	return
}

func (self Result) Success(data interface{}) {
	if IsQuiet() {
		return
	}
	str, err := json.Marshal(data)
	if err != nil {
		self.Error(err)
		return
	}
	fmt.Println(string(str))
}
