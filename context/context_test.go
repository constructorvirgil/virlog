package context

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/virlog/logger"
)

// 测试GetFromContext函数
func TestGetFromContext(t *testing.T) {
	// 创建基础上下文
	ctx := context.Background()

	// 测试从空上下文获取Logger
	log1 := GetFromContext(ctx)
	assert.NotNil(t, log1, "从空上下文中获取Logger不应该返回nil")
	assert.Equal(t, logger.DefaultLogger(), log1, "从空上下文应返回默认Logger")

	// 测试从nil上下文获取Logger
	log2 := GetFromContext(nil)
	assert.NotNil(t, log2, "从nil上下文获取Logger不应返回nil")
	assert.Equal(t, logger.DefaultLogger(), log2, "从nil上下文应返回默认Logger")
}

// 测试SaveToContext函数
func TestSaveToContext(t *testing.T) {
	// 创建基础上下文
	ctx := context.Background()

	// 创建测试Logger
	testLogger := logger.With(logger.String("test", "value"))

	// 将Logger保存到上下文
	ctx = SaveToContext(ctx, testLogger)

	// 验证能否从上下文获取Logger
	logFromCtx := GetFromContext(ctx)
	assert.Equal(t, testLogger, logFromCtx, "应该从上下文中获取到相同的Logger")

	// 测试nil上下文和nil Logger
	nilCtx := SaveToContext(nil, nil)
	assert.NotNil(t, nilCtx, "创建的上下文不应为nil")

	// 验证从nil上下文创建的上下文中获取Logger
	logFromNilCtx := GetFromContext(nilCtx)
	assert.Equal(t, logger.DefaultLogger(), logFromNilCtx, "应该使用默认Logger")
}

// 测试WithFields函数
func TestWithFields(t *testing.T) {
	// 创建基础上下文
	ctx := context.Background()

	// 添加一组字段
	ctx, log1 := WithFields(ctx, logger.String("key1", "value1"))

	// 再添加一组字段
	_, log2 := WithFields(ctx, logger.String("key2", "value2"))

	// 验证log2与log1不同
	assert.NotEqual(t, log1, log2, "添加字段后应该返回新的Logger实例")

	// 验证全新上下文的日志字段
	newCtx := context.Background()
	_, log3 := WithFields(newCtx, logger.String("key3", "value3"))

	// log3应与log1、log2不同
	assert.NotEqual(t, log1, log3, "不同上下文的Logger应该不同")
	assert.NotEqual(t, log2, log3, "不同上下文的Logger应该不同")
}

// 测试三个函数的组合使用
func TestCombinedUsage(t *testing.T) {
	// 创建基础上下文
	ctx := context.Background()

	// 1. 使用WithFields添加字段
	ctx, log1 := WithFields(ctx, logger.String("app", "test-app"))

	// 2. 创建一个新Logger并保存到上下文
	customLogger := logger.With(logger.String("module", "auth"))
	ctx = SaveToContext(ctx, customLogger)

	// 3. 从上下文获取Logger，应该是customLogger
	log2 := GetFromContext(ctx)
	assert.Equal(t, customLogger, log2, "应该获取到SaveToContext保存的Logger")
	assert.NotEqual(t, log1, log2, "SaveToContext应覆盖之前WithFields添加的Logger")

	// 4. 再次使用WithFields添加字段
	_, log3 := WithFields(ctx, logger.String("request_id", "12345"))

	// 验证log3是从log2派生的，而不是从log1
	assert.NotEqual(t, log1, log3, "WithFields应该从当前上下文中的Logger派生")
}
