/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
