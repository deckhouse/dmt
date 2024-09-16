package logger

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
)

var logger *slog.Logger

func InitLogger() {
	log.SetOutput(io.Discard)
	slog.SetLogLoggerLevel(slog.LevelInfo)
	logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
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
