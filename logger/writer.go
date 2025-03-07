package logger

import (
	"io"

	"github.com/virlog/config"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// RotateWriter 支持日志轮转的io.Writer实现
type RotateWriter struct {
	*lumberjack.Logger
}

// NewRotateWriter 创建一个新的日志轮转writer
func NewRotateWriter(cfg *config.FileConfig) io.Writer {
	if cfg == nil {
		cfg = config.DefaultConfig().FileConfig
	}
	return &RotateWriter{
		Logger: &lumberjack.Logger{
			Filename:   cfg.Filename,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		},
	}
}

// Write 实现io.Writer接口
func (w *RotateWriter) Write(p []byte) (n int, err error) {
	return w.Logger.Write(p)
}

// CustomWriter 自定义的writer接口
type CustomWriter interface {
	io.Writer
	Sync() error
}

// MultiWriter 创建多输出writer
func MultiWriter(writers ...io.Writer) zapcore.WriteSyncer {
	return zapcore.AddSync(io.MultiWriter(writers...))
}
