package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sleet0922/graduation_project/internal/config"
)

// 全局实例
var Log *slog.Logger

// 从viper读取配置,初始化
func InitLogger(cfg *config.ViperConfig) {
	// 日志相关配置(文件路径、级别)
	logConfig := cfg.Log
	// 判断是否开发环境
	isDev := cfg.Server.Mode != "release"
	// 默认日志级别
	var logLevel slog.Level

	// 日志级别
	switch logConfig.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// 创建日志文件夹 + 文件
	err := os.MkdirAll(filepath.Dir(logConfig.Filename), os.ModePerm)
	if err != nil {
		panic("创建日志目录失败：" + err.Error())
	}
	logFile, err := os.OpenFile(logConfig.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("打开日志文件失败: " + err.Error())
	}

	cwd, _ := os.Getwd()
	opts := &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source, ok := a.Value.Any().(*slog.Source)
				if ok && source != nil {
					if rel, err := filepath.Rel(cwd, source.File); err == nil {
						source.File = rel
					}
				}
			}
			return a
		},
	}

	var handler slog.Handler
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	if isDev {
		// 开发环境：文本格式
		opts.AddSource = false
		handler = slog.NewTextHandler(multiWriter, opts)
	} else {
		// 生产环境：JSON格式
		opts.AddSource = true
		handler = slog.NewJSONHandler(multiWriter, opts)
	}

	Log = slog.New(handler)
	slog.SetDefault(Log)
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
