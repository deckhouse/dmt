package logger

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/deckhouse/d8-lint/pkg/flags"
)

var logger *slog.Logger

func InitLogger() {
	log.SetOutput(io.Discard)

	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)

	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))

	if flags.LogLevel == "DEBUG" {
		lvl.Set(slog.LevelDebug)
	}
	if flags.LogLevel == "INFO" {
		lvl.Set(slog.LevelInfo)
	}
	if flags.LogLevel == "WARN" {
		lvl.Set(slog.LevelWarn)
	}
	if flags.LogLevel == "ERROR" {
		lvl.Set(slog.LevelError)
	}
}

func DebugF(format string, a ...any) {
	logger.Debug(
		fmt.Sprintf(format, a...))
}

func InfoF(format string, a ...any) {
	logger.Info(
		fmt.Sprintf(format, a...))
}

func WarnF(format string, a ...any) {
	logger.Warn(
		fmt.Sprintf(format, a...))
}

func ErrorF(format string, a ...any) {
	logger.Error(
		fmt.Sprintf(format, a...))
}

func CheckErr(msg any) {
	if msg != nil {
		logger.Error(
			fmt.Sprintf("Error: %s", msg))
		os.Exit(1)
	}
}
