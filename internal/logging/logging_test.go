package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{"debug": slog.LevelDebug, "info": slog.LevelInfo, "warn": slog.LevelWarn, "warning": slog.LevelWarn, "error": slog.LevelError}
	for input, want := range tests {
		got, err := ParseLevel(input)
		if err != nil || got != want {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v", input, got, err, want)
		}
	}
	if _, err := ParseLevel("trace"); err == nil {
		t.Fatal("ParseLevel(trace) should return an error")
	}
}

func TestNewFiltersByLevel(t *testing.T) {
	var output bytes.Buffer
	logger := New(&output, slog.LevelWarn)
	logger.Debugf("debug message")
	logger.Infof("info message")
	logger.Warnf("warn message")
	logger.Errorf("error message")
	text := output.String()
	if strings.Contains(text, "debug message") || strings.Contains(text, "info message") {
		t.Fatalf("low-level messages were not filtered: %s", text)
	}
	if !strings.Contains(text, "warn message") || !strings.Contains(text, "error message") {
		t.Fatalf("warn/error messages missing: %s", text)
	}
}
