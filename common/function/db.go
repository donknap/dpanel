package function

import (
	"context"
	"database/sql/driver"
	"fmt"
	"reflect"
	"slices"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type SQLLogger struct {
	logger.Interface
}

func (self SQLLogger) LogMode(level logger.LogLevel) logger.Interface {
	self.Interface = self.Interface.LogMode(level)
	return self
}

func (self SQLLogger) ParamsFilter(ctx context.Context, sql string, paramList ...any) (string, []any) {
	// 日志参数可能与执行参数共用切片，只修改展示副本。
	paramList = slices.Clone(paramList)
	if filter, ok := self.Interface.(gorm.ParamsFilter); ok {
		sql, paramList = filter.ParamsFilter(ctx, sql, paramList...)
	}
	for i, param := range paramList {
		if value, ok := param.(driver.Valuer); ok {
			// JSON 序列化字段在这里仍是 Valuer，展开后才能限制展示长度。
			if reflect.ValueOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil() {
				param = nil
			} else {
				param, _ = value.Value()
			}
		}
		switch value := param.(type) {
		case string:
			param = truncateSQLLog(value, 1024)
		case []byte:
			if len(value) > 1024 {
				param = []byte(truncateSQLLog(string(value), 1024))
			}
		}
		paramList[i] = param
	}
	return sql, paramList
}

func (self SQLLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	self.Interface.Trace(ctx, begin, func() (string, int64) {
		sql, rowCount := fc()
		return truncateSQLLog(sql, 64*1024), rowCount
	}, err)
}

func truncateSQLLog(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	suffix := fmt.Sprintf("...[truncated, original %d bytes]", len(value))
	end := limit - len(suffix)
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + suffix
}
