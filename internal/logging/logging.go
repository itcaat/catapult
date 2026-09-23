package logging

import (
	"fmt"
	"io"
	"log/slog"
)

// Logger is the application logging facade. The formatted helpers are kept
// temporarily for the existing call sites while all output is handled by
// slog and filtered by level.
type Logger struct {
	*slog.Logger
}

func New(w io.Writer, level slog.Level) *Logger {
	return &Logger{Logger: slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))}
}

func From(logger *slog.Logger) *Logger { return &Logger{Logger: logger} }

func (l *Logger) Debugf(format string, args ...any) { l.Debug(fmt.Sprintf(format, args...)) }
func (l *Logger) Infof(format string, args ...any)  { l.Info(fmt.Sprintf(format, args...)) }
func (l *Logger) Warnf(format string, args ...any)  { l.Warn(fmt.Sprintf(format, args...)) }
func (l *Logger) Errorf(format string, args ...any) { l.Error(fmt.Sprintf(format, args...)) }

// ParseLevel converts the CLI value into a slog level.
func ParseLevel(value string) (slog.Level, error) {
	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level %q (use debug, info, warn, or error)", value)
	}
}
