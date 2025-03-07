package context

import (
	"context"

	"github.com/virlog/logger"
)

// 定义上下文key类型，用于从上下文提取日志字段
type loggerKey struct{}

// WithContext 从上下文中提取字段，如果没有则创建新的Logger
func WithContext(ctx context.Context) logger.Logger {
	if ctx == nil {
		return logger.DefaultLogger()
	}
	if ctxLogger, ok := ctx.Value(loggerKey{}).(logger.Logger); ok {
		return ctxLogger
	}
	return logger.DefaultLogger()
}

// NewContext 在上下文中添加Logger
func NewContext(ctx context.Context, log logger.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = logger.DefaultLogger()
	}
	return context.WithValue(ctx, loggerKey{}, log)
}

// WithFields 向上下文中的Logger添加字段
func WithFields(ctx context.Context, fields ...logger.Field) (context.Context, logger.Logger) {
	log := WithContext(ctx).With(fields...)
	return NewContext(ctx, log), log
}

// LoggerFromContext 从上下文中获取Logger（别名方法）
func LoggerFromContext(ctx context.Context) logger.Logger {
	return WithContext(ctx)
}

// ContextWithLogger 向上下文中添加Logger（别名方法）
func ContextWithLogger(ctx context.Context, log logger.Logger) context.Context {
	return NewContext(ctx, log)
}
