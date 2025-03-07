package context

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/virlog/logger"
)

// 测试上下文功能
func TestLoggerContext(t *testing.T) {
	// 创建基础上下文
	ctx := context.Background()

	// 测试从空上下文获取Logger
	log1 := WithContext(ctx)
	assert.NotNil(t, log1, "从空上下文中获取Logger不应该返回nil")

	// 测试向上下文添加Logger
	testLogger := logger.With(logger.String("test", "value"))
	ctx = NewContext(ctx, testLogger)

	// 测试从上下文中获取Logger
	log2 := WithContext(ctx)
	assert.Equal(t, testLogger, log2, "应该从上下文中获取到相同的Logger")

	// 测试向上下文中Logger添加字段
	ctx, log3 := WithFields(ctx, logger.Int("id", 123), logger.String("name", "test"))
	assert.NotEqual(t, log2, log3, "添加字段后应该是不同的Logger实例")

	// 测试别名方法
	log4 := LoggerFromContext(ctx)
	assert.Equal(t, log3, log4, "LoggerFromContext应该返回相同的Logger")

	ctx = ContextWithLogger(ctx, testLogger)
	log5 := WithContext(ctx)
	assert.Equal(t, testLogger, log5, "ContextWithLogger应该正确设置Logger")
}

// 测试空上下文
func TestNilContext(t *testing.T) {
	// 测试nil上下文
	log := WithContext(nil)
	assert.NotNil(t, log, "从nil上下文获取Logger不应返回nil")
	assert.Equal(t, logger.DefaultLogger(), log, "从nil上下文应返回默认Logger")

	// 测试nil上下文创建
	ctx := NewContext(nil, nil)
	assert.NotNil(t, ctx, "创建的上下文不应为nil")

	// 验证上下文中的Logger
	log = WithContext(ctx)
	assert.Equal(t, logger.DefaultLogger(), log, "应该使用默认Logger")
}

// 测试字段添加
func TestWithFields(t *testing.T) {
	ctx := context.Background()

	// 添加一组字段
	ctx, log1 := WithFields(ctx, logger.String("key1", "value1"))

	// 再添加一组字段
	ctx, log2 := WithFields(ctx, logger.String("key2", "value2"))

	// 验证log2与log1不同
	assert.NotEqual(t, log1, log2, "添加字段后应该返回新的Logger实例")

	// 验证全新上下文的日志字段
	newCtx := context.Background()
	_, log3 := WithFields(newCtx, logger.String("key3", "value3"))

	// log3应与log1、log2不同
	assert.NotEqual(t, log1, log3, "不同上下文的Logger应该不同")
	assert.NotEqual(t, log2, log3, "不同上下文的Logger应该不同")
}
