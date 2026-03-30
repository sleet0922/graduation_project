package logger

import (
	"os"
	"path/filepath"
	"sleet0922/graduation_project/internal/config"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 全局实例
var Log *zap.Logger

// 从viper读取配置,初始化
func InitLogger(cfg *config.ViperConfig) {
	// 日志相关配置(文件路径、级别)
	logConfig := cfg.Log
	// 判断是否开发环境
	isDev := cfg.Server.Mode != "release"
	// 默认日志级别
	logLevel := zapcore.InfoLevel

	// 日志级别
	switch logConfig.Level {
	case "debug":
		logLevel = zapcore.DebugLevel
	case "info":
		logLevel = zapcore.InfoLevel
	case "warn":
		logLevel = zapcore.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	}

	// 配置日志样式
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05"))
	}

	// 选择日志样式
	var encoder zapcore.Encoder
	if isDev {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// 创建日志文件夹+文件
	if err := os.MkdirAll(filepath.Dir(logConfig.Filename), os.ModePerm); err != nil {
		panic("创建日志目录失败: " + err.Error())
	}
	logFile, err := os.OpenFile(logConfig.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("打开日志文件失败: " + err.Error())
	}

	// 输出到控制台+文件
	writeSyncer := zapcore.NewMultiWriteSyncer(
		zapcore.AddSync(os.Stdout),
		zapcore.AddSync(logFile),
	)

	// 8. 创建zap核心对象
	core := zapcore.NewCore(encoder, writeSyncer, logLevel)
	if isDev {
		Log = zap.New(core)
	} else {
		Log = zap.New(core, zap.AddCaller())
	}

	// 替换zap全局logger
	zap.ReplaceGlobals(Log)
}

// 日志级别函数
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}
