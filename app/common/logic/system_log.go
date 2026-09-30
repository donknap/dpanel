package logic

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	dockerEvents "github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

var panelLogPattern = regexp.MustCompile(`^\[([^]]+)]\s+\[([^]]+)]\s+(.*)$`)

const MaxSystemLogItems = 500
const maxPanelLogBytes = 8 << 20
const panelLogBackupTimeFormat = "2006-01-02T15-04-05.000"

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

func (self SystemLog) ReadDockerEvents() []SystemLogItem {
	list := make([]SystemLogItem, 0)
	if value, ok := storage.Cache.Get(storage.CacheKeyDockerEvents); ok {
		if cached, ok := value.([]*event.DockerMessagePayload); ok {
			list = make([]SystemLogItem, 0, len(cached))
			for _, item := range cached {
				if item == nil {
					continue
				}
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
		}
	}
	return list
}

func (self SystemLog) upgradeLogFilePaths() ([]string, error) {
	directory := filepath.Join(storage.Local{}.GetStorageLocalPath(), "logs")
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() ||
			!strings.HasPrefix(name, "upgrade-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		timestamp := strings.TrimSuffix(strings.TrimPrefix(name, "upgrade-"), ".log")
		if _, err := time.ParseInLocation(define.DateYmdHis, timestamp, time.Local); err != nil {
			continue
		}
		paths = append(paths, filepath.Join(directory, name))
	}
	sort.Strings(paths)
	return paths, nil
}

func (self SystemLog) ReadLatestUpgrade() ([]SystemLogItem, error) {
	paths, err := self.upgradeLogFilePaths()
	if err != nil || len(paths) == 0 {
		return []SystemLogItem{}, err
	}
	path := paths[len(paths)-1]
	filename := filepath.Base(path)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 8<<20)

	result := make([]SystemLogItem, 0)
	for scanner.Scan() {
		line := scanner.Text()
		var record struct {
			Time  time.Time `json:"time"`
			Level string    `json:"level"`
			Msg   string    `json:"msg"`
		}
		if json.Unmarshal([]byte(line), &record) != nil || record.Time.IsZero() {
			continue
		}
		item := SystemLogItem{
			ID:          fmt.Sprintf("upgrade:%s:%d", filename, len(result)),
			Source:      "upgrade",
			SourceName:  "upgrade",
			SourceType:  "upgrade",
			Level:       types.LogLevelInfo,
			Description: record.Msg,
			CreatedAt:   record.Time.UnixMilli(),
		}
		switch strings.ToLower(record.Level) {
		case "debug":
			item.Level = types.LogLevelDebug
		case "warn", "warning":
			item.Level = types.LogLevelWarning
		case "error":
			item.Level = types.LogLevelError
		}
		var attributes map[string]json.RawMessage
		if json.Unmarshal([]byte(line), &attributes) == nil {
			delete(attributes, "time")
			delete(attributes, "level")
			delete(attributes, "msg")
			if len(attributes) > 0 {
				encoded, err := json.Marshal(attributes)
				if err != nil {
					return nil, err
				}
				if item.Description != "" {
					item.Description += "\n"
				}
				item.Description += string(encoded)
			}
		}
		result = append(result, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (self SystemLog) PruneOldUpgrades() error {
	paths, err := self.upgradeLogFilePaths()
	if err != nil {
		return err
	}
	for _, path := range paths[:max(0, len(paths)-1)] {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

func (self SystemLog) ReadPanel() ([]SystemLogItem, error) {
	path := self.PanelLogFilePath()
	if path == "" {
		return []SystemLogItem{}, nil
	}

	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []SystemLogItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	position := max(int64(0), info.Size()-maxPanelLogBytes)
	result := make([]SystemLogItem, 0, MaxSystemLogItems)
	next := 0
	lastIndex := -1
	itemIndex := 0
	var description strings.Builder
	scanner := bufio.NewScanner(io.NewSectionReader(file, position, info.Size()-position))
	scanner.Buffer(make([]byte, 64*1024), maxPanelLogBytes+1)
	for scanner.Scan() {
		line := scanner.Text()
		match := panelLogPattern.FindStringSubmatch(line)
		if len(match) != 4 {
			if lastIndex >= 0 && strings.TrimSpace(line) != "" {
				description.WriteByte('\n')
				description.WriteString(line)
			}
			continue
		}

		createdAt, err := time.ParseInLocation("2006-01-02 15:04:05.000", match[1], time.Local)
		if err != nil {
			continue
		}
		level := types.LogLevel(strings.ToLower(strings.TrimSpace(match[2])))
		if level == "warn" {
			level = types.LogLevelWarning
		}
		if level != types.LogLevelDebug && level != types.LogLevelInfo &&
			level != types.LogLevelWarning && level != types.LogLevelError {
			continue
		}
		message := match[3]
		if _, value, exists := strings.Cut(message, "\t"); exists {
			message = value
		}
		if lastIndex >= 0 {
			result[lastIndex].Description = description.String()
			description.Reset()
		}
		description.WriteString(strings.TrimSpace(message))
		item := SystemLogItem{
			ID:         fmt.Sprintf("panel:%d:%d", createdAt.UnixMilli(), itemIndex),
			Source:     "panel",
			SourceName: "panel",
			SourceType: "panel",
			Level:      level,
			CreatedAt:  createdAt.UnixMilli(),
		}
		itemIndex++
		if len(result) < MaxSystemLogItems {
			result = append(result, item)
			lastIndex = len(result) - 1
		} else {
			result[next] = item
			lastIndex = next
			next = (next + 1) % MaxSystemLogItems
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	if lastIndex >= 0 {
		result[lastIndex].Description = description.String()
	}
	if next > 0 {
		ordered := make([]SystemLogItem, 0, len(result))
		ordered = append(ordered, result[next:]...)
		result = append(ordered, result[:next]...)
	}
	return result, nil
}

func (self SystemLog) PanelLogFilePath() string {
	path := strings.TrimSpace(facade.GetConfig().GetString("log.file.path"))
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join("runtime", "logs", path)
	}
	return filepath.Clean(path)
}

func (self SystemLog) PrunePanel() error {
	path := self.PanelLogFilePath()
	if path != "" {
		if _, err := facade.GetLoggerFactory().Channel("file"); err != nil {
			return err
		}
		if err := facade.GetLoggerFactory().Rotate("file"); err != nil {
			return err
		}
		directory := filepath.Dir(path)
		entries, err := os.ReadDir(directory)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
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
				return err
			}
		}
	}
	return nil
}
