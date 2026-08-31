package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sleet0922/graduation_project/internal/config"
)

// 全局实例
var Log *slog.Logger
var activeCloser io.Closer

type closeFunc struct {
	once sync.Once
	fn   func() error
	err  error
}

func (f *closeFunc) Close() error {
	f.once.Do(func() { f.err = f.fn() })
	return f.err
}

// New creates a configured logger without changing the process-wide default.
// The returned closer releases the log file when the application shuts down.
func New(cfg *config.ViperConfig) (*slog.Logger, io.Closer, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("logger config is nil")
	}

	logFilename := strings.TrimSpace(cfg.Log.Filename)
	if logFilename == "" {
		return nil, nil, fmt.Errorf("logger filename is empty")
	}
	logDir := filepath.Dir(logFilename)
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return nil, nil, fmt.Errorf("create logger directory %q: %w", logDir, err)
	}
	logFile, err := os.OpenFile(logFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return nil, nil, fmt.Errorf("open logger file %q: %w", logFilename, err)
	}

	opts := &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// 移除 level 字段
			if a.Key == slog.LevelKey {
				return slog.Attr{}
			}
			// 修改时间格式
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("2006/01/02 15:04:05"))
				}
			}
			// 修改 source 路径为相对路径（从/pkg开始）
			if a.Key == slog.SourceKey {
				source, ok := a.Value.Any().(*slog.Source)
				if ok && source != nil {
					if idx := strings.Index(source.File, "/pkg"); idx != -1 {
						source.File = source.File[idx:]
					}
					if idx := strings.Index(source.Function, "/pkg"); idx != -1 {
						source.Function = source.Function[idx:]
					}
				}
			}
			return a
		},
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Log.Level)) {
	case "", "info":
		opts.Level = slog.LevelInfo
	case "debug":
		opts.Level = slog.LevelDebug
	case "warn", "warning":
		opts.Level = slog.LevelWarn
	case "error":
		opts.Level = slog.LevelError
	default:
		_ = logFile.Close()
		return nil, nil, fmt.Errorf("invalid logger level %q", cfg.Log.Level)
	}

	var slogHandler slog.Handler
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	if !strings.EqualFold(strings.TrimSpace(cfg.Server.Mode), "release") {
		opts.AddSource = false
		slogHandler = slog.NewTextHandler(multiWriter, opts)
	} else {
		opts.AddSource = true
		slogHandler = slog.NewJSONHandler(multiWriter, opts)
	}

	return slog.New(slogHandler), &closeFunc{fn: logFile.Close}, nil
}

// Setup creates and installs the process-wide logger used by legacy handlers.
// New code should prefer passing a *slog.Logger explicitly where practical.
func Setup(cfg *config.ViperConfig) (io.Closer, error) {
	configured, closer, err := New(cfg)
	if err != nil {
		return nil, err
	}
	if activeCloser != nil {
		_ = activeCloser.Close()
	}
	Log = configured
	slog.SetDefault(configured)
	activeCloser = closer
	return closer, nil
}

// Close releases the currently configured log file, if any.
func Close() error {
	if activeCloser == nil {
		return nil
	}
	err := activeCloser.Close()
	activeCloser = nil
	return err
}

// 从viper读取配置,初始化
func InitLogger(cfg *config.ViperConfig) error {
	closer, err := Setup(cfg)
	if err != nil {
		return err
	}
	// The application owns the closer through Setup. Keep this wrapper for
	// callers that only need process-wide initialization.
	_ = closer
	return nil
}

// 日志级别函数
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}
func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
