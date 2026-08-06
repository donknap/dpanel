package crontab

import "log/slog"

type cronLogger struct {
	name string
}

func (self cronLogger) Info(message string, keysAndValues ...interface{}) {
	args := []any{"name", self.name, "event", message}
	args = append(args, keysAndValues...)
	slog.Info("cron job", args...)
}

func (self cronLogger) Error(err error, message string, keysAndValues ...interface{}) {
	args := []any{"name", self.name, "event", message, "error", err}
	args = append(args, keysAndValues...)
	slog.Warn("cron job", args...)
}
