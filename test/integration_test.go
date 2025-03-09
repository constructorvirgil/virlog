package test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/constructorvirgil/virlog/config"
	logctx "github.com/constructorvirgil/virlog/context"
	"github.com/constructorvirgil/virlog/logger"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

// TestContextLoggerIntegration 测试context和logger配合使用
func TestContextLoggerIntegration(t *testing.T) {
	// 创建内存缓冲区作为日志输出目标
	buf := &bytes.Buffer{}

	// 创建logger配置
	cfg := config.DefaultConfig()
	cfg.Level = "debug"
	cfg.Format = "json" // 使用JSON格式便于解析验证

	// 创建WriteSyncer
	ws := zapcore.AddSync(buf)

	// 使用WithSyncTarget选项创建logger
	baseLogger, err := logger.NewLogger(cfg, logger.WithSyncTarget(ws))
	assert.NoError(t, err, "创建logger失败")

	// 创建基础context
	ctx := context.Background()

	// 将logger保存到context中
	ctx = logctx.SaveToContext(ctx, baseLogger)

	// 模拟请求处理流程
	handleRequest(ctx, t, buf)
}

// handleRequest 模拟请求处理函数
func handleRequest(ctx context.Context, t *testing.T, buf *bytes.Buffer) {
	// 从context中获取logger
	log := logctx.GetFromContext(ctx)
	assert.NotNil(t, log, "从context中获取logger不应为nil")

	// 记录请求开始
	log.Info("开始处理请求")

	// 记录buf中当前内容，用于后续验证
	currentOutput := buf.String()
	logLines := strings.Split(strings.TrimSpace(currentOutput), "\n")
	assert.Equal(t, 1, len(logLines), "应该有1条日志记录")

	// 解析第一条日志
	var firstLog map[string]interface{}
	err := json.Unmarshal([]byte(logLines[0]), &firstLog)
	assert.NoError(t, err, "解析日志失败")
	assert.Equal(t, "开始处理请求", firstLog["msg"])

	// 添加请求ID
	logWithRequestID := log.With(logger.String("request_id", "req-12345"))

	// 更新context中的logger
	ctx = logctx.SaveToContext(ctx, logWithRequestID)

	// 调用认证处理
	authenticate(ctx, t, buf)
}

// authenticate 模拟认证处理
func authenticate(ctx context.Context, t *testing.T, buf *bytes.Buffer) {
	// 从context获取logger (应该包含request_id)
	log := logctx.GetFromContext(ctx)

	// 记录认证开始
	log.Info("开始用户认证")

	// 添加用户信息
	logWithUser := log.With(logger.String("user_id", "user-789"), logger.String("role", "admin"))

	// 更新context
	ctx = logctx.SaveToContext(ctx, logWithUser)

	// 调用业务逻辑处理
	processBusinessLogic(ctx, t, buf)
}

// processBusinessLogic 模拟业务逻辑处理
func processBusinessLogic(ctx context.Context, t *testing.T, buf *bytes.Buffer) {
	// 从context获取logger (应该包含request_id, user_id, role)
	log := logctx.GetFromContext(ctx)

	// 记录业务处理开始
	log.Info("开始处理业务逻辑")

	// 记录详细操作
	log.With(
		logger.String("action", "update_profile"),
		logger.Int("affected_rows", 1),
	).Info("更新用户资料")

	// 记录处理完成
	log.Info("业务逻辑处理完成")

	// 获取并验证所有日志输出
	validateLogs(t, buf)
}

// validateLogs 验证所有日志内容
func validateLogs(t *testing.T, buf *bytes.Buffer) {
	// 获取完整输出
	output := buf.String()

	// 分割多行日志
	logLines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 5, len(logLines), "应该有5条日志记录")

	// 解析各条日志
	var logs []map[string]interface{}
	for i, line := range logLines {
		var log map[string]interface{}
		err := json.Unmarshal([]byte(line), &log)
		assert.NoError(t, err, "解析第%d条日志失败", i+1)
		logs = append(logs, log)
	}

	// 验证第一条日志 - 开始处理请求
	assert.Equal(t, "开始处理请求", logs[0]["msg"])
	assert.Nil(t, logs[0]["request_id"]) // 第一条日志没有request_id

	// 验证第二条日志 - 开始用户认证
	assert.Equal(t, "开始用户认证", logs[1]["msg"])
	assert.Equal(t, "req-12345", logs[1]["request_id"])
	assert.Nil(t, logs[1]["user_id"]) // 还没有用户ID

	// 验证第三条日志 - 开始处理业务逻辑
	assert.Equal(t, "开始处理业务逻辑", logs[2]["msg"])
	assert.Equal(t, "req-12345", logs[2]["request_id"])
	assert.Equal(t, "user-789", logs[2]["user_id"])
	assert.Equal(t, "admin", logs[2]["role"])

	// 验证第四条日志 - 更新用户资料
	assert.Equal(t, "更新用户资料", logs[3]["msg"])
	assert.Equal(t, "req-12345", logs[3]["request_id"])
	assert.Equal(t, "user-789", logs[3]["user_id"])
	assert.Equal(t, "admin", logs[3]["role"])
	assert.Equal(t, "update_profile", logs[3]["action"])
	assert.Equal(t, float64(1), logs[3]["affected_rows"]) // JSON将数字解析为float64

	// 验证第五条日志 - 业务逻辑处理完成
	assert.Equal(t, "业务逻辑处理完成", logs[4]["msg"])
	assert.Equal(t, "req-12345", logs[4]["request_id"])
	assert.Equal(t, "user-789", logs[4]["user_id"])
	assert.Equal(t, "admin", logs[4]["role"])
}

// TestContextWithFieldsIntegration 测试WithFields方法
func TestContextWithFieldsIntegration(t *testing.T) {
	// 创建内存缓冲区
	buf := &bytes.Buffer{}

	// 创建logger配置
	cfg := config.DefaultConfig()
	cfg.Level = "debug"
	cfg.Format = "json"

	// 创建logger
	baseLogger, err := logger.NewLogger(cfg, logger.WithSyncTarget(zapcore.AddSync(buf)))
	assert.NoError(t, err, "创建logger失败")

	// 创建基础context
	ctx := context.Background()

	// 将logger保存到context
	ctx = logctx.SaveToContext(ctx, baseLogger)

	// 添加字段到context中的logger
	ctx, _ = logctx.WithFields(ctx,
		logger.String("trace_id", "trace-abc123"),
		logger.String("service", "user-service"),
	)

	// 获取logger并记录日志
	log := logctx.GetFromContext(ctx)
	log.Info("第一条日志")

	// 再次添加字段
	ctx, _ = logctx.WithFields(ctx,
		logger.String("method", "GET"),
		logger.String("path", "/api/users"),
	)

	// 再次获取logger并记录日志
	log = logctx.GetFromContext(ctx)
	log.Info("第二条日志")

	// 在当前日志上添加临时字段(不保存到context)
	log.With(logger.Int("status", 200)).Info("请求完成")

	// 验证日志内容
	output := buf.String()
	logLines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 3, len(logLines), "应该有3条日志记录")

	// 解析日志
	var logs []map[string]interface{}
	for i, line := range logLines {
		var log map[string]interface{}
		err := json.Unmarshal([]byte(line), &log)
		assert.NoError(t, err, "解析第%d条日志失败", i+1)
		logs = append(logs, log)
	}

	// 验证第一条日志
	assert.Equal(t, "第一条日志", logs[0]["msg"])
	assert.Equal(t, "trace-abc123", logs[0]["trace_id"])
	assert.Equal(t, "user-service", logs[0]["service"])
	assert.Nil(t, logs[0]["method"])

	// 验证第二条日志 - 应该包含所有累积的字段
	assert.Equal(t, "第二条日志", logs[1]["msg"])
	assert.Equal(t, "trace-abc123", logs[1]["trace_id"])
	assert.Equal(t, "user-service", logs[1]["service"])
	assert.Equal(t, "GET", logs[1]["method"])
	assert.Equal(t, "/api/users", logs[1]["path"])

	// 验证第三条日志 - 应该包含所有字段加上临时的status字段
	assert.Equal(t, "请求完成", logs[2]["msg"])
	assert.Equal(t, "trace-abc123", logs[2]["trace_id"])
	assert.Equal(t, "user-service", logs[2]["service"])
	assert.Equal(t, "GET", logs[2]["method"])
	assert.Equal(t, "/api/users", logs[2]["path"])
	assert.Equal(t, float64(200), logs[2]["status"]) // JSON中的数字解析为float64
}
