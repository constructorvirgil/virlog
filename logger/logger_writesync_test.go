package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/virlog/config"
	"go.uber.org/zap/zapcore"
)

// TestLogToMemoryBuffer 测试日志输出到内存缓冲区并验证内容
func TestLogToMemoryBuffer(t *testing.T) {
	// 创建内存缓冲区
	buf := &bytes.Buffer{}

	// 创建配置
	cfg := config.DefaultConfig()
	cfg.Level = "debug"
	cfg.Format = "json" // 使用JSON格式便于解析验证

	// 创建WriteSyncer
	ws := zapcore.AddSync(buf)

	// 使用WithSyncTarget选项创建logger
	logger, err := NewLogger(cfg, WithSyncTarget(ws))
	assert.NoError(t, err, "创建logger失败")

	// 输出不同级别的日志
	logger.Debug("这是一条调试日志", String("key1", "value1"))
	logger.Info("这是一条信息日志", Int("count", 42))
	logger.Warn("这是一条警告日志", Bool("active", true))

	// 验证输出内容
	output := buf.String()

	// 分割多行日志
	logLines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 3, len(logLines), "应该有3条日志记录")

	// 解析并验证每条日志
	var debugLog, infoLog, warnLog map[string]interface{}

	// 解析Debug日志
	err = json.Unmarshal([]byte(logLines[0]), &debugLog)
	assert.NoError(t, err, "解析debug日志失败")
	assert.Equal(t, "debug", debugLog["level"])
	assert.Equal(t, "这是一条调试日志", debugLog["msg"])
	assert.Equal(t, "value1", debugLog["key1"])

	// 解析Info日志
	err = json.Unmarshal([]byte(logLines[1]), &infoLog)
	assert.NoError(t, err, "解析info日志失败")
	assert.Equal(t, "info", infoLog["level"])
	assert.Equal(t, "这是一条信息日志", infoLog["msg"])
	assert.Equal(t, float64(42), infoLog["count"]) // JSON将数字解析为float64

	// 解析Warn日志
	err = json.Unmarshal([]byte(logLines[2]), &warnLog)
	assert.NoError(t, err, "解析warn日志失败")
	assert.Equal(t, "warn", warnLog["level"])
	assert.Equal(t, "这是一条警告日志", warnLog["msg"])
	assert.Equal(t, true, warnLog["active"])
}

// TestLoggerWithField 测试使用With方法添加字段
func TestLoggerWithField(t *testing.T) {
	// 创建内存缓冲区
	buf := &bytes.Buffer{}

	// 创建配置
	cfg := config.DefaultConfig()
	cfg.Level = "debug"
	cfg.Format = "json"

	// 创建WriteSyncer
	ws := zapcore.AddSync(buf)

	// 创建logger
	logger, err := NewLogger(cfg, WithSyncTarget(ws))
	assert.NoError(t, err, "创建logger失败")

	// 使用With方法创建衍生logger
	derivedLogger := logger.With(
		String("requestID", "req-123"),
		String("userID", "user-456"),
	)

	// 使用衍生logger记录日志
	derivedLogger.Info("处理用户请求")

	// 再次添加字段
	derivedLogger.With(String("action", "login")).Info("用户登录")

	// 验证输出
	output := buf.String()
	logLines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 2, len(logLines), "应该有2条日志记录")

	// 解析日志
	var firstLog, secondLog map[string]interface{}

	// 第一条日志应包含requestID和userID
	err = json.Unmarshal([]byte(logLines[0]), &firstLog)
	assert.NoError(t, err, "解析第一条日志失败")
	assert.Equal(t, "处理用户请求", firstLog["msg"])
	assert.Equal(t, "req-123", firstLog["requestID"])
	assert.Equal(t, "user-456", firstLog["userID"])

	// 第二条日志应包含所有字段
	err = json.Unmarshal([]byte(logLines[1]), &secondLog)
	assert.NoError(t, err, "解析第二条日志失败")
	assert.Equal(t, "用户登录", secondLog["msg"])
	assert.Equal(t, "req-123", secondLog["requestID"])
	assert.Equal(t, "user-456", secondLog["userID"])
	assert.Equal(t, "login", secondLog["action"])
}

// TestMultiSyncTargets 测试多个输出目标
func TestMultiSyncTargets(t *testing.T) {
	// 创建两个内存缓冲区
	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}

	// 创建多输出WriteSyncer
	multiWS := zapcore.AddSync(zapcore.NewMultiWriteSyncer(
		zapcore.AddSync(buf1),
		zapcore.AddSync(buf2),
	))

	// 创建配置
	cfg := config.DefaultConfig()
	cfg.Level = "info"
	cfg.Format = "json"

	// 创建logger
	logger, err := NewLogger(cfg, WithSyncTarget(multiWS))
	assert.NoError(t, err, "创建logger失败")

	// 记录日志
	logger.Info("这条日志应该同时输出到两个缓冲区")

	// 验证两个缓冲区内容一致
	output1 := buf1.String()
	output2 := buf2.String()

	assert.NotEmpty(t, output1, "buf1应该有内容")
	assert.Equal(t, output1, output2, "两个缓冲区内容应该一致")

	// 解析日志内容
	var log map[string]interface{}
	err = json.Unmarshal([]byte(strings.TrimSpace(output1)), &log)
	assert.NoError(t, err, "解析日志失败")
	assert.Equal(t, "这条日志应该同时输出到两个缓冲区", log["msg"])
	assert.Equal(t, "info", log["level"])
}
