package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/virlog/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 测试配置创建
func TestDefaultLogger(t *testing.T) {
	logger := DefaultLogger()
	assert.NotNil(t, logger, "默认Logger不应为nil")
}

// 测试创建日志器
func TestNewLoggerWithDefaultConfig(t *testing.T) {
	logger, err := NewLogger(nil)

	assert.NoError(t, err)
	assert.NotNil(t, logger)
}

// 测试自定义配置
func TestNewLoggerWithCustomConfig(t *testing.T) {
	cfg := &config.Config{
		Level:            "debug",
		Format:           "console",
		Output:           "stdout",
		Development:      true,
		EnableCaller:     true,
		EnableStacktrace: false,
		DefaultFields: map[string]interface{}{
			"service": "test-service",
		},
	}

	logger, err := NewLogger(cfg)

	assert.NoError(t, err)
	assert.NotNil(t, logger)
}

// 创建测试用的buffer输出日志器
func newBufferLogger(level Level) (*zapLogger, *bytes.Buffer) {
	buf := &bytes.Buffer{}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	encoder := zapcore.NewJSONEncoder(encoderConfig)
	writeSyncer := zapcore.AddSync(buf)
	atom := zap.NewAtomicLevelAt(level)

	core := zapcore.NewCore(encoder, writeSyncer, atom)
	zapLoggerInst := zap.New(core)

	return &zapLogger{
		zapLogger: zapLoggerInst,
		atom:      &atom,
		config:    config.DefaultConfig(),
		fields:    make([]Field, 0),
	}, buf
}

// 测试日志输出
func TestLoggerLevels(t *testing.T) {
	logger, buf := newBufferLogger(DebugLevel)

	tests := []struct {
		name     string
		logFunc  func(string, ...Field)
		level    string
		message  string
		expected bool
	}{
		{"Debug", logger.Debug, "debug", "debug message", true},
		{"Info", logger.Info, "info", "info message", true},
		{"Warn", logger.Warn, "warn", "warn message", true},
		{"Error", logger.Error, "error", "error message", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf.Reset()
			test.logFunc(test.message)

			if test.expected {
				logData := make(map[string]interface{})
				err := json.Unmarshal(buf.Bytes(), &logData)
				require.NoError(t, err)

				assert.Equal(t, test.message, logData["msg"])
				assert.Equal(t, test.level, logData["level"])
			} else {
				assert.Empty(t, buf.String())
			}
		})
	}
}

// 测试日志级别过滤
func TestLogLevelFiltering(t *testing.T) {
	logger, buf := newBufferLogger(WarnLevel)

	// Debug级别信息不应输出
	logger.Debug("debug message")
	assert.Empty(t, buf.String())

	// Info级别信息不应输出
	buf.Reset()
	logger.Info("info message")
	assert.Empty(t, buf.String())

	// Warn级别信息应输出
	buf.Reset()
	logger.Warn("warn message")
	assert.NotEmpty(t, buf.String())

	// Error级别信息应输出
	buf.Reset()
	logger.Error("error message")
	assert.NotEmpty(t, buf.String())
}

// 测试With方法
func TestLoggerWith(t *testing.T) {
	logger, buf := newBufferLogger(InfoLevel)

	childLogger := logger.With(String("key", "value"))
	childLogger.Info("test message")

	logData := make(map[string]interface{})
	err := json.Unmarshal(buf.Bytes(), &logData)
	require.NoError(t, err)

	assert.Equal(t, "test message", logData["msg"])
	assert.Equal(t, "value", logData["key"])
}

// 测试SetLevel方法
func TestLoggerSetLevel(t *testing.T) {
	logger, buf := newBufferLogger(InfoLevel)

	// Debug级别信息不应输出
	logger.Debug("debug message")
	assert.Empty(t, buf.String())

	// 修改级别为Debug
	logger.SetLevel(DebugLevel)

	// Debug级别信息应输出
	buf.Reset()
	logger.Debug("debug message")
	assert.NotEmpty(t, buf.String())
}

// 测试文件输出
func TestFileOutput(t *testing.T) {
	// 创建临时文件名
	tempFile := "temp_test.log"
	defer os.Remove(tempFile)

	// 配置文件输出
	cfg := &config.Config{
		Level:  "info",
		Format: "json",
		Output: "file",
		FileConfig: &config.FileConfig{
			Filename:   tempFile,
			MaxSize:    1,
			MaxBackups: 1,
			MaxAge:     1,
			Compress:   false,
		},
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)

	// 写入日志
	logger.Info("test file output")
	logger.Sync()

	// 验证文件写入
	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.NotEmpty(t, content)

	// 验证日志内容
	logData := make(map[string]interface{})
	err = json.Unmarshal(content, &logData)
	require.NoError(t, err)
	assert.Equal(t, "test file output", logData["msg"])
}

// 测试全局函数
func TestGlobalFunctions(t *testing.T) {
	// 保存原始的std logger
	originalStd := std
	defer func() {
		std = originalStd
	}()

	// 创建测试logger
	logger, buf := newBufferLogger(InfoLevel)

	// 设置为默认logger
	SetDefault(logger)

	// 测试全局函数
	Info("global info message")

	logData := make(map[string]interface{})
	err := json.Unmarshal(buf.Bytes(), &logData)
	require.NoError(t, err)

	assert.Equal(t, "global info message", logData["msg"])
	assert.Equal(t, "info", logData["level"])
}
