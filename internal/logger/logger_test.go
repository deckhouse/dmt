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
	"bytes"
	"log/slog"
	"testing"
)

func TestInitLogger(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		expected string
	}{
		{"DebugLevel", DebugLogLevel, "DEBUG"},
		{"InfoLevel", InfoLogLevel, "INFO"},
		{"WarnLevel", WarnLogLevel, "WARN"},
		{"ErrorLevel", ErrorLogLevel, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.Buffer{}
			InitLogger(&buf, tt.logLevel)
			slog.Debug("test debug")
			slog.Info("test info")
			slog.Warn("test warn")
			slog.Error("test error")

			if !bytes.Contains(buf.Bytes(), []byte(tt.expected)) {
				t.Errorf("expected log level %s not found in output", tt.expected)
			}
			buf.Reset()
		})
	}
}

func TestLogFunctions(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{}))
	slog.SetDefault(logger)

	DebugF("Debug message: %d", 1)
	InfoF("Info message: %s", "test")
	WarnF("Warn message")
	ErrorF("Error message")

	logOutput := buf.String()
	tests := []struct {
		name     string
		expected string
	}{
		{"DebugMessage", "Debug message: 1"},
		{"InfoMessage", "Info message: test"},
		{"WarnMessage", "Warn message"},
		{"ErrorMessage", "Error message"},
	}

	for _, tt := range tests {
		if !bytes.Contains([]byte(logOutput), []byte(tt.expected)) {
			t.Errorf("expected log message %q not found in output", tt.expected)
		}
	}
}
