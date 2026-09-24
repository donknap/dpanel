package controller

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/donknap/dpanel/app/common/events"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/types"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

const panelLogBackupTimeFormat = "2006-01-02T15-04-05.000"

type Log struct {
	controller.Abstract
}

func (self Log) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Page     int              `json:"page" binding:"omitempty,gt=0"`
		PageSize int              `json:"pageSize" binding:"omitempty,gt=0"`
		Keyword  string           `json:"keyword"`
		Level    []types.LogLevel `json:"level" binding:"omitempty,dive,oneof=debug info warning error"`
		Source   []string         `json:"source" binding:"omitempty,dive,required"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 10
	}

	list, err := (logic.SystemLog{}).Collect(logic.SystemLogQuery{
		Keyword: params.Keyword,
		Level:   params.Level,
		Source:  params.Source,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	total := len(list)
	offset := (params.Page - 1) * params.PageSize
	if offset >= total {
		list = []logic.SystemLogItem{}
	} else {
		end := offset + params.PageSize
		if end > total {
			end = total
		}
		list = list[offset:end]
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list":  list,
		"page":  params.Page,
		"total": total,
	})
}

func (self Log) Download(httpContext *gin.Context) {
	type ParamsValidate struct {
		Keyword string   `json:"keyword"`
		ID      []string `json:"id" binding:"omitempty,dive,required"`
	}

	params := ParamsValidate{}
	if !self.Validate(httpContext, &params) {
		return
	}
	query := logic.SystemLogQuery{ID: params.ID}
	if len(params.ID) == 0 {
		query.Keyword = params.Keyword
	}
	list, err := (logic.SystemLog{}).Collect(query)
	if err != nil {
		self.JsonResponseWithError(httpContext, err, http.StatusInternalServerError)
		return
	}

	var content strings.Builder
	for index, item := range list {
		if index > 0 {
			content.WriteByte('\n')
		}
		_, _ = fmt.Fprintf(
			&content,
			"[%s] [%s] [%s] %s",
			time.UnixMilli(item.CreatedAt).Format("2006-01-02 15:04:05.000"),
			item.Level,
			item.Source,
			item.Content(),
		)
	}
	filename := "dpanel-system-log-" + time.Now().Format("20060102-150405") + ".log"
	httpContext.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	httpContext.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content.String()))
}

func (self Log) Prune(httpContext *gin.Context) {
	path := strings.TrimSpace(facade.GetConfig().GetString("log.file.path"))
	if path != "" {
		if !filepath.IsAbs(path) {
			path = filepath.Join("runtime", "logs", path)
		}
		path = filepath.Clean(path)
		if _, err := facade.GetLoggerFactory().Channel("file"); err != nil {
			self.JsonResponseWithError(httpContext, err, http.StatusInternalServerError)
			return
		}
		if err := facade.GetLoggerFactory().Rotate("file"); err != nil {
			self.JsonResponseWithError(httpContext, err, http.StatusInternalServerError)
			return
		}
		directory := filepath.Dir(path)
		entries, err := os.ReadDir(directory)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			self.JsonResponseWithError(httpContext, err, http.StatusInternalServerError)
			return
		}

		filename := filepath.Base(path)
		extension := filepath.Ext(filename)
		prefix := strings.TrimSuffix(filename, extension) + "-"
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
				continue
			}
			suffix := extension
			if strings.HasSuffix(entry.Name(), extension+".gz") {
				suffix = extension + ".gz"
			} else if !strings.HasSuffix(entry.Name(), extension) {
				continue
			}
			timestamp := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), prefix), suffix)
			if _, err = time.Parse(panelLogBackupTimeFormat, timestamp); err != nil {
				continue
			}
			if err = os.Remove(filepath.Join(directory, entry.Name())); err != nil {
				self.JsonResponseWithError(httpContext, err, http.StatusInternalServerError)
				return
			}
		}
	}
	(events.Docker{}).ClearMessages()
	self.JsonSuccessResponse(httpContext)
	return
}
