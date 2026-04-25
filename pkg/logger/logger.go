package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// 创建日志文件夹 + 文件
	err := os.MkdirAll(filepath.Dir(logConfig.Filename), os.ModePerm)
	if err != nil {
		panic("创建日志目录失败：" + err.Error())
	}
	logFile, err := os.OpenFile(logConfig.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("打开日志文件失败: " + err.Error())
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
					// 查找 /pkg 在路径中的位置
					if idx := strings.Index(source.File, "/pkg"); idx != -1 {
						source.File = source.File[idx:]
					}
					// 处理 function 路径
					if idx := strings.Index(source.Function, "/pkg"); idx != -1 {
						source.Function = source.Function[idx:]
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
