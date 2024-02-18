package controller

import (
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type RunEnv struct {
	controller.Abstract
}

func (self RunEnv) Create(http *gin.Context) {

}

func (self RunEnv) SupportRunEnv(http *gin.Context) {
	env := make(map[string][]string)
	env[logic.LangPhp] = []string{
		"php56",
		"php72", // alpine:3.9
		"php74", // alpine:3.15
		"php81", // alpine:3.18
	}
	env[logic.LangJava] = []string{
		"jdk8",
		"jdk12",
		"jdk18",
	}
	env[logic.LangGolang] = []string{
		"go",
	}
	env[logic.LangNode] = []string{
		"node12",
		"node14",
		"node18", // node:14-alpine
		"node20",
	}
	env[logic.LangHtml] = []string{
		"nginx",
	}
	env[logic.LangOther] = []string{
		"alpine",
		"ubuntu",
	}
	self.JsonResponseWithoutError(http, env)
	return
}

func (self RunEnv) PhpExt(http *gin.Context) {
	self.JsonResponseWithoutError(http, gin.H{
		"ext": []string{
			"php-pecl-memcached", "php-pecl-redis", "php-opcache", "php-pecl-imagick", "php-exif",
			"php-intl", "php-pecl-apcu", "php-imap", "php-pecl-mongodb", "php-pdo_pgsql", "php-pecl-swoole",
		},
	})
	return
}
