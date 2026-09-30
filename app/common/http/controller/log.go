package controller

import (
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	dockerEvents "github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/app/common/events"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/types"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Log struct {
	controller.Abstract
}

func (self Log) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Keyword string           `json:"keyword"`
		Level   []types.LogLevel `json:"level" binding:"omitempty,dive,oneof=debug info warning error"`
		Source  []string         `json:"source" binding:"omitempty,dive,required"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	panelList, err := (logic.SystemLog{}).ReadPanel()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	dockerList := (logic.SystemLog{}).ReadDockerEvents()
	list := make([]logic.SystemLogItem, 0, len(panelList)+len(dockerList))
	list = append(list, panelList...)
	list = append(list, dockerList...)
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].CreatedAt > list[j].CreatedAt
	})
	if len(list) > logic.MaxSystemLogItems {
		list = list[:logic.MaxSystemLogItems]
	}
	upgradeLogs, err := (logic.SystemLog{}).ReadLatestUpgrade()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if len(upgradeLogs) > 0 {
		insertAt := sort.Search(len(list), func(i int) bool {
			return list[i].CreatedAt <= upgradeLogs[0].CreatedAt
		})
		merged := make([]logic.SystemLogItem, 0, len(list)+len(upgradeLogs))
		merged = append(merged, list[:insertAt]...)
		merged = append(merged, upgradeLogs...)
		list = append(merged, list[insertAt:]...)
	}

	levelFilter := make(map[types.LogLevel]struct{}, len(params.Level))
	for _, level := range params.Level {
		levelFilter[level] = struct{}{}
	}
	sourceFilter := make(map[string]struct{}, len(params.Source))
	for _, source := range params.Source {
		sourceFilter[source] = struct{}{}
	}

	var keywordLevel types.LogLevel
	var keywordSource string
	keyword := strings.TrimSpace(params.Keyword)
	filter, remainder, hasKeyword := strings.Cut(keyword, ":")
	levelValue, sourceValue, hasSource := strings.Cut(filter, "@")
	if hasSource && strings.TrimSpace(sourceValue) != "" || !hasSource && hasKeyword {
		validLevel := true
		switch strings.ToLower(strings.TrimSpace(levelValue)) {
		case "":
			validLevel = hasSource
		case "debug":
			keywordLevel = types.LogLevelDebug
		case "info":
			keywordLevel = types.LogLevelInfo
		case "warn", "warning":
			keywordLevel = types.LogLevelWarning
		case "error", "dpanic", "panic", "fatal":
			keywordLevel = types.LogLevelError
		default:
			validLevel = false
		}
		if validLevel {
			if hasSource {
				keywordSource = strings.TrimSpace(sourceValue)
			}
			keyword = ""
			if hasKeyword {
				keyword = strings.TrimSpace(remainder)
			}
		}
	}
	lowerKeyword := strings.ToLower(keyword)

	filtered := make([]logic.SystemLogItem, 0, len(list))
	for _, item := range list {
		if len(levelFilter) > 0 {
			if _, ok := levelFilter[item.Level]; !ok {
				continue
			}
		}
		if keywordLevel != "" && item.Level != keywordLevel {
			continue
		}
		if keywordSource != "" &&
			!strings.EqualFold(item.SourceName, keywordSource) &&
			!strings.EqualFold(item.SourceType, keywordSource) {
			continue
		}
		if len(sourceFilter) > 0 {
			_, sourceMatched := sourceFilter[item.Source]
			_, sourceTypeMatched := sourceFilter[item.SourceType]
			if !sourceMatched && !sourceTypeMatched {
				continue
			}
		}
		if lowerKeyword != "" {
			content := item.Description
			if item.Message != nil {
				message := item.Message
				target := message.Actor.Attributes["name"]
				if target == "" {
					target = message.Actor.ID
				}
				if target == "" {
					target = message.From
				}
				if target == "" {
					target = message.ID
				}
				if target == "" {
					target = "-"
				}
				containerID := message.Actor.Attributes["container"]
				if containerID != "" && (message.Type == dockerEvents.NetworkEventType || message.Type == dockerEvents.VolumeEventType) {
					container := containerID
					if containerName := message.Actor.Attributes["containerName"]; containerName != "" {
						container = containerName + " (" + containerID + ")"
					}
					content = string(message.Type) + " " + target + " container " + container + ": " + string(message.Action)
				} else {
					content = string(message.Type) + " " + target + ": " + string(message.Action)
				}
			}
			if !strings.Contains(strings.ToLower(content), lowerKeyword) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list":  filtered,
		"total": len(filtered),
	})
}

func (self Log) Download(httpContext *gin.Context) {
	path := (logic.SystemLog{}).PanelLogFilePath()
	if _, err := os.Stat(path); err != nil {
		self.JsonResponseWithError(httpContext, err, http.StatusInternalServerError)
		return
	}
	downloadUrl, err := (logic.Attach{}).PreDownload(path, time.Second*10)
	if err != nil {
		self.JsonResponseWithError(httpContext, err, http.StatusInternalServerError)
		return
	}
	self.JsonResponseWithoutError(httpContext, gin.H{"downloadUrl": downloadUrl})
}

func (self Log) Prune(httpContext *gin.Context) {
	if err := (logic.SystemLog{}).PrunePanel(); err != nil {
		self.JsonResponseWithError(httpContext, err, http.StatusInternalServerError)
		return
	}
	if err := (logic.SystemLog{}).PruneOldUpgrades(); err != nil {
		self.JsonResponseWithError(httpContext, err, http.StatusInternalServerError)
		return
	}
	(events.Docker{}).ClearMessages()
	self.JsonSuccessResponse(httpContext)
}
