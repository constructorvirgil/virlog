package context

import (
	"context"

	"github.com/constructorvirgil/virlog/logger"
)

// 定义上下文key类型，用于从上下文提取日志字段
type loggerKey struct{}

// GetFromContext 从上下文中提取Logger，如果没有则返回默认Logger
func GetFromContext(ctx context.Context) logger.Logger {
	if ctx == nil {
		return logger.DefaultLogger()
	}
	if ctxLogger, ok := ctx.Value(loggerKey{}).(logger.Logger); ok {
		return ctxLogger
	}
	return logger.DefaultLogger()
}

// SaveToContext 在上下文中添加Logger
func SaveToContext(ctx context.Context, log logger.Logger) context.Context {
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
	log := GetFromContext(ctx).With(fields...)
	return SaveToContext(ctx, log), log
}
