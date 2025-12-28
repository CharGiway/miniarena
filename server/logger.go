package server

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Log 是全局可用的 SugaredLogger，用于统一日志输出到文件
var Log *zap.SugaredLogger

// InitLogger 初始化 zap 日志到本地文件（支持滚动）
// filePath: 日志文件路径，如 "app.log"
func InitLogger(filePath string) error {
	// 文件滚动策略：10MB 每文件，保留3个备份，按天备份可选
	lj := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    10, // MB
		MaxBackups: 3,
		MaxAge:     7, // days
		Compress:   false,
	}

	ws := zapcore.AddSync(lj)
	encCfg := zapcore.EncoderConfig{
		TimeKey:       "ts",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "stack",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}
	// 控制台风格更易读，也可改为 JSON：zapcore.NewJSONEncoder(encCfg)
	encoder := zapcore.NewConsoleEncoder(encCfg)
	core := zapcore.NewCore(encoder, ws, zapcore.DebugLevel) // Debug 级别，便于排查

	// 添加调用者信息（文件:行号）
	logger := zap.New(core, zap.AddCaller())
	Log = logger.Sugar()
	return nil
}

// SyncLogger 清理和同步缓冲
func SyncLogger() {
	if Log != nil {
		_ = Log.Sync()
	}
}
