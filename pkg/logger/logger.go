package logger

import (
	"fmt"
	"log/slog"
)

func Infof(format string, a ...any) {
	slog.Info(
		fmt.Sprintf(format, a...))
}

func Warnf(format string, a ...any) {
	slog.Warn(
		fmt.Sprintf(format, a...))
}
