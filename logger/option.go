package logger

import "go.uber.org/zap/zapcore"

// Option 定义logger选项的函数类型
type Option func(*zapLogger)

// WithSyncTarget 设置自定义的同步输出目标
func WithSyncTarget(syncTarget zapcore.WriteSyncer) Option {
	return func(l *zapLogger) {
		// 将syncTarget应用到logger的配置中
		l.syncTarget = syncTarget
	}
}
