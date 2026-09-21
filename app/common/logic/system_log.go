package logic

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	dockerEvents "github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

var panelLogPattern = regexp.MustCompile(`^\[([^]]+)]\s+\[([^]]+)]\s+(.*)$`)

type PanelLog struct {
	Level     types.LogLevel
	Message   string
	CreatedAt int64
}

type SystemLogQuery struct {
	ID      []string
	Keyword string
	Level   []types.LogLevel
	Source  []string
}

type SystemLogItem struct {
	ID          string                `json:"id"`
	Source      string                `json:"source"`
	SourceName  string                `json:"sourceName"`
	SourceType  string                `json:"sourceType"`
	Level       types.LogLevel        `json:"level"`
	Description string                `json:"description,omitempty"`
	Message     *dockerEvents.Message `json:"message,omitempty"`
	CreatedAt   int64                 `json:"createdAt"`
}

type SystemLog struct {
}

func (self SystemLog) Collect(query SystemLogQuery) ([]SystemLogItem, error) {
	panelList, err := self.ReadPanel()
	if err != nil {
		return nil, err
	}

	dockerList := make([]event.DockerMessagePayload, 0)
	if value, ok := storage.Cache.Get(storage.CacheKeyDockerEvents); ok {
		if cached, ok := value.([]*event.DockerMessagePayload); ok {
			dockerList = make([]event.DockerMessagePayload, 0, len(cached))
			for _, item := range cached {
				if item != nil {
					dockerList = append(dockerList, *item)
				}
			}
		}
	}
	list := make([]SystemLogItem, 0, len(panelList)+len(dockerList))
	for index, item := range panelList {
		list = append(list, SystemLogItem{
			ID:          fmt.Sprintf("panel:%d:%d", item.CreatedAt, index),
			Source:      "panel",
			SourceName:  "panel",
			SourceType:  "panel",
			Level:       item.Level,
			Description: item.Message,
			CreatedAt:   item.CreatedAt,
		})
	}
	for _, item := range dockerList {
		createdAt := item.Message.TimeNano / int64(time.Millisecond)
		if createdAt == 0 {
			createdAt = item.Message.Time * int64(time.Second/time.Millisecond)
		}
		message := item.Message
		list = append(list, SystemLogItem{
			ID:         item.ID,
			Source:     "docker:" + item.DockerEnvName,
			SourceName: item.DockerEnvName,
			SourceType: "docker",
			Level:      item.Level,
			Message:    &message,
			CreatedAt:  createdAt,
		})
	}

	levelFilter := make(map[types.LogLevel]struct{}, len(query.Level))
	for _, level := range query.Level {
		levelFilter[level] = struct{}{}
	}
	sourceFilter := make(map[string]struct{}, len(query.Source))
	for _, source := range query.Source {
		sourceFilter[source] = struct{}{}
	}
	idFilter := make(map[string]struct{}, len(query.ID))
	for _, id := range query.ID {
		idFilter[id] = struct{}{}
	}
	keywordLevel, keywordSource, keyword := parseSystemLogKeyword(query.Keyword)
	lowerKeyword := strings.ToLower(keyword)

	result := make([]SystemLogItem, 0, len(list))
	for _, item := range list {
		if len(idFilter) > 0 {
			if _, ok := idFilter[item.ID]; !ok {
				continue
			}
		}
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
		if lowerKeyword != "" && !strings.Contains(strings.ToLower(item.Content()), lowerKeyword) {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].CreatedAt > result[j].CreatedAt
	})
	return result, nil
}

func (item SystemLogItem) Content() string {
	if item.Message == nil {
		return item.Description
	}
	return dockerMessageContent(*item.Message)
}

func (self SystemLog) ReadPanel() ([]PanelLog, error) {
	path := panelLogFilePath()
	if path == "" {
		return []PanelLog{}, nil
	}

	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []PanelLog{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	result := make([]PanelLog, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		match := panelLogPattern.FindStringSubmatch(line)
		if len(match) != 4 {
			if len(result) > 0 && strings.TrimSpace(line) != "" {
				result[len(result)-1].Message += "\n" + line
			}
			continue
		}

		createdAt, err := time.ParseInLocation("2006-01-02 15:04:05.000", match[1], time.Local)
		if err != nil {
			continue
		}
		level, ok := parsePanelLogLevel(match[2])
		if !ok {
			continue
		}
		message := match[3]
		if _, value, exists := strings.Cut(message, "\t"); exists {
			message = value
		}
		result = append(result, PanelLog{
			Level:     level,
			Message:   strings.TrimSpace(message),
			CreatedAt: createdAt.UnixMilli(),
		})
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func panelLogFilePath() string {
	path := strings.TrimSpace(facade.GetConfig().GetString("log.file.path"))
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join("runtime", "logs", path)
	}
	return filepath.Clean(path)
}

func parsePanelLogLevel(value string) (types.LogLevel, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return types.LogLevelDebug, true
	case "info":
		return types.LogLevelInfo, true
	case "warn", "warning":
		return types.LogLevelWarning, true
	case "error", "dpanic", "panic", "fatal":
		return types.LogLevelError, true
	default:
		return "", false
	}
}

func parseSystemLogKeyword(value string) (types.LogLevel, string, string) {
	value = strings.TrimSpace(value)
	filter, keyword, hasKeyword := strings.Cut(value, ":")
	if levelValue, source, exists := strings.Cut(filter, "@"); exists {
		source = strings.TrimSpace(source)
		if source == "" {
			return "", "", value
		}
		level, ok := parsePanelLogLevel(levelValue)
		if strings.TrimSpace(levelValue) != "" && !ok {
			return "", "", value
		}
		if !hasKeyword {
			keyword = ""
		}
		return level, source, strings.TrimSpace(keyword)
	}
	if !hasKeyword {
		return "", "", value
	}

	level, ok := parsePanelLogLevel(filter)
	if !ok {
		return "", "", value
	}
	return level, "", strings.TrimSpace(keyword)
}

func dockerMessageContent(message dockerEvents.Message) string {
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
			container = fmt.Sprintf("%s (%s)", containerName, containerID)
		}
		return fmt.Sprintf("%s %s container %s: %s", message.Type, target, container, message.Action)
	}
	return fmt.Sprintf("%s %s: %s", message.Type, target, message.Action)
}
