package logic

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gookit/color"
)

type Result struct{}

func (self Result) Error(err error) {
	str, err := json.Marshal(gin.H{
		"error": err.Error(),
		"code":  500,
	})
	if err != nil {
		self.Error(err)
		return
	}
	color.Errorln("\033c", string(str))
	return
}

func (self Result) Success(data interface{}) {
	str, err := json.Marshal(data)
	if err != nil {
		self.Error(err)
		return
	}
	color.Successln("\033c", string(str))
}
