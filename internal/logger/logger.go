package logger

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
)

func InitLogger(logLevel string) {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))

	if logLevel == "DEBUG" {
		lvl.Set(slog.LevelDebug)
	}
	if logLevel == "INFO" {
		lvl.Set(slog.LevelInfo)
	}
	if logLevel == "WARN" {
		lvl.Set(slog.LevelWarn)
	}
	if logLevel == "ERROR" {
		lvl.Set(slog.LevelError)
	}

	slog.SetDefault(logger)
	log.SetOutput(io.Discard)
}

func DebugF(format string, a ...any) {
	slog.Debug(
		fmt.Sprintf(format, a...))
}

func InfoF(format string, a ...any) {
	slog.Info(
		fmt.Sprintf(format, a...))
}

func WarnF(format string, a ...any) {
	slog.Warn(
		fmt.Sprintf(format, a...))
}

func ErrorF(format string, a ...any) {
	slog.Error(
		fmt.Sprintf(format, a...))
}

func CheckErr(msg any) {
	if msg != nil {
		slog.Error(
			fmt.Sprintf("Error: %s", msg))
		os.Exit(1)
	}
}
