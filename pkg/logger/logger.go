package logger

import (
	"fmt"
	"log/slog"
	"os"
)

var logger *slog.Logger

func InitLogger() {
	slog.SetLogLoggerLevel(slog.LevelInfo)
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func Infof(format string, a ...any) {
	logger.Info(
		fmt.Sprintf(format, a...))
}

func Warnf(format string, a ...any) {
	logger.Warn(
		fmt.Sprintf(format, a...))
}

func CheckErr(msg any) {
	if msg != nil {
		logger.Error(
			fmt.Sprintf("Error: %s", msg))
		os.Exit(1)
	}
}
