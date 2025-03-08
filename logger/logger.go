package logger

import (
	"os"
	"sync"
	"time"

	"github.com/virlog/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Field 是日志字段类型
type Field = zapcore.Field

// 预定义的字段构造函数
var (
	// 基本类型
	Binary  = zap.Binary
	Bool    = zap.Bool
	String  = zap.String
	Int     = zap.Int
	Int64   = zap.Int64
	Float64 = zap.Float64
	Err     = zap.Error
	Any     = zap.Any

	// 其他常用类型
	Namespace = zap.Namespace
	Reflect   = zap.Reflect
	Skip      = zap.Skip
	Time      = zap.Time
	Duration  = zap.Duration
)

// 日志级别
type Level = zapcore.Level

const (
	// 从低到高排序
	DebugLevel  = zapcore.DebugLevel
	InfoLevel   = zapcore.InfoLevel
	WarnLevel   = zapcore.WarnLevel
	ErrorLevel  = zapcore.ErrorLevel
	DPanicLevel = zapcore.DPanicLevel // 开发模式下会panic
	PanicLevel  = zapcore.PanicLevel  // 日志后会panic
	FatalLevel  = zapcore.FatalLevel  // 日志后会os.Exit(1)
)

// Logger 定义日志接口
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	DPanic(msg string, fields ...Field)
	Panic(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	// 支持层级日志记录
	With(fields ...Field) Logger

	// 支持动态修改日志级别
	SetLevel(level Level)

	// 同步刷新所有缓存的日志
	Sync() error

	// 获取原始zap logger
	GetRawZapLogger() *zap.Logger
}

// 确保 zapLogger 实现了 Logger 接口
var _ Logger = (*zapLogger)(nil)

// Option 定义logger选项的函数类型
type Option func(*zapLogger)

// WithSyncTarget 设置自定义的同步输出目标
func WithSyncTarget(syncTarget zapcore.WriteSyncer) Option {
	return func(l *zapLogger) {
		// 将syncTarget应用到logger的配置中
		l.syncTarget = syncTarget
	}
}

// zapLogger 是对 zap.Logger 的封装
type zapLogger struct {
	rawZapLogger *zap.Logger
	atom         *zap.AtomicLevel
	config       *config.Config
	fields       []Field
	mu           sync.RWMutex
	syncTarget   zapcore.WriteSyncer // 自定义的同步输出目标
}

// getZapLevel 将配置中的日志级别字符串转换为zap日志级别
func getZapLevel(levelStr string) zapcore.Level {
	switch levelStr {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "dpanic":
		return DPanicLevel
	case "panic":
		return PanicLevel
	case "fatal":
		return FatalLevel
	default:
		return InfoLevel
	}
}

// getEncoderConfig 获取编码器配置
func getEncoderConfig(cfg *config.Config) zapcore.EncoderConfig {
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

	if cfg.Development {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeCaller = zapcore.FullCallerEncoder
	}

	return encoderConfig
}

// getEncoder 获取日志编码器
func getEncoder(encoderConfig zapcore.EncoderConfig, cfg *config.Config) zapcore.Encoder {
	if cfg.Format == "console" {
		return zapcore.NewConsoleEncoder(encoderConfig)
	}
	return zapcore.NewJSONEncoder(encoderConfig)
}

// getOutputConfig 获取输出配置
func getOutputConfig(cfg *config.Config) (zapcore.WriteSyncer, error) {
	var writeSyncer zapcore.WriteSyncer
	switch cfg.Output {
	case "stdout":
		writeSyncer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writeSyncer = zapcore.AddSync(os.Stderr)
	case "file":
		if cfg.FileConfig == nil {
			cfg.FileConfig = config.DefaultConfig().FileConfig
		}
		lumberjackLogger := &lumberjack.Logger{
			Filename:   cfg.FileConfig.Filename,
			MaxSize:    cfg.FileConfig.MaxSize,
			MaxBackups: cfg.FileConfig.MaxBackups,
			MaxAge:     cfg.FileConfig.MaxAge,
			Compress:   cfg.FileConfig.Compress,
		}
		writeSyncer = zapcore.AddSync(lumberjackLogger)
	default:
		writeSyncer = zapcore.AddSync(os.Stdout)
	}
	return writeSyncer, nil
}

// NewLogger 创建一个新的Logger实例
func NewLogger(cfg *config.Config, opts ...Option) (Logger, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// 默认level是DEBUG
	atom := zap.NewAtomicLevelAt(getZapLevel(cfg.Level))

	// 创建zapLogger实例
	logger := &zapLogger{
		atom:   &atom,
		config: cfg,
		fields: make([]Field, 0),
	}

	// 应用所有选项
	for _, opt := range opts {
		opt(logger)
	}

	// 获取encoder配置
	encoderConfig := getEncoderConfig(cfg)

	// 获取输出配置
	var writeSyncer zapcore.WriteSyncer
	var err error
	if logger.syncTarget != nil {
		// 如果设置了自定义同步目标，使用它
		writeSyncer = logger.syncTarget
	} else {
		// 否则使用默认配置
		writeSyncer, err = getOutputConfig(cfg)
		if err != nil {
			return nil, err
		}
	}

	// 从配置中读取预设字段
	var fields []Field
	for k, v := range cfg.DefaultFields {
		// 根据类型进行转换
		switch val := v.(type) {
		case string:
			fields = append(fields, String(k, val))
		case int:
			fields = append(fields, Int(k, val))
		case int64:
			fields = append(fields, Int64(k, val))
		case float64:
			fields = append(fields, Float64(k, val))
		case bool:
			fields = append(fields, Bool(k, val))
		default:
			fields = append(fields, Any(k, val))
		}
	}

	// 创建核心
	core := zapcore.NewCore(
		getEncoder(encoderConfig, cfg),
		writeSyncer,
		atom,
	)

	// 创建zap logger
	rawZapLogger := zap.New(core, getZapOptions(cfg)...).With(fields...)

	// 保存到zapLogger实例
	logger.rawZapLogger = rawZapLogger

	return logger, nil
}

// getZapOptions 返回zap配置选项
func getZapOptions(cfg *config.Config) []zap.Option {
	var options []zap.Option

	if cfg.EnableCaller {
		options = append(options, zap.AddCaller())
	}

	if cfg.EnableStacktrace {
		options = append(options, zap.AddStacktrace(ErrorLevel))
	}

	if cfg.Development {
		options = append(options, zap.Development())
	}

	if cfg.EnableSampling {
		options = append(options, zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(
				core,
				time.Second,
				100,
				100,
			)
		}))
	}

	return options
}

// Debug 输出Debug级别日志
func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.rawZapLogger.Debug(msg, fields...)
}

// Info 输出Info级别日志
func (l *zapLogger) Info(msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.rawZapLogger.Info(msg, fields...)
}

// Warn 输出Warn级别日志
func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.rawZapLogger.Warn(msg, fields...)
}

// Error 输出Error级别日志
func (l *zapLogger) Error(msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.rawZapLogger.Error(msg, fields...)
}

// DPanic 输出DPanic级别日志
func (l *zapLogger) DPanic(msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.rawZapLogger.DPanic(msg, fields...)
}

// Panic 输出Panic级别日志并触发panic
func (l *zapLogger) Panic(msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.rawZapLogger.Panic(msg, fields...)
}

// Fatal 输出Fatal级别日志并调用os.Exit(1)
func (l *zapLogger) Fatal(msg string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.rawZapLogger.Fatal(msg, fields...)
}

// With 返回带有指定字段的新Logger
func (l *zapLogger) With(fields ...Field) Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	allFields := append(l.fields, fields...)
	return &zapLogger{
		rawZapLogger: l.rawZapLogger.With(fields...),
		atom:         l.atom,
		config:       l.config,
		fields:       allFields,
		syncTarget:   l.syncTarget,
	}
}

// SetLevel 动态修改日志级别
func (l *zapLogger) SetLevel(level Level) {
	l.atom.SetLevel(level)
}

// Sync 将缓冲的日志刷新到输出
func (l *zapLogger) Sync() error {
	return l.rawZapLogger.Sync()
}

// GetZapLogger 返回原始zap.Logger
func (l *zapLogger) GetRawZapLogger() *zap.Logger {
	return l.rawZapLogger
}

var (
	std Logger
	mu  sync.RWMutex
)

// init 初始化全局Logger
func init() {
	var err error
	std, err = NewLogger(config.GetConfig())
	if err != nil {
		panic("failed to initialize global logger: " + err.Error())
	}

	// 启动配置监听
	go watchConfig()
}

// 监听配置变更
func watchConfig() {
	// 创建配置变更监听器
	configChan := make(chan *config.Config, 1)
	config.AddListener(configChan)

	// 监听配置变更
	for cfg := range configChan {
		// 创建新的logger
		newLogger, err := NewLogger(cfg)
		if err != nil {
			// 配置变更失败，继续使用旧配置
			continue
		}

		// 更新全局logger
		SetDefault(newLogger)
	}
}

// 全局函数，使用默认Logger

// Debug 使用默认Logger输出Debug级别日志
func Debug(msg string, fields ...Field) {
	std.Debug(msg, fields...)
}

// Info 使用默认Logger输出Info级别日志
func Info(msg string, fields ...Field) {
	std.Info(msg, fields...)
}

// Warn 使用默认Logger输出Warn级别日志
func Warn(msg string, fields ...Field) {
	std.Warn(msg, fields...)
}

// Error 使用默认Logger输出Error级别日志
func Error(msg string, fields ...Field) {
	std.Error(msg, fields...)
}

// DPanic 使用默认Logger输出DPanic级别日志
func DPanic(msg string, fields ...Field) {
	std.DPanic(msg, fields...)
}

// Panic 使用默认Logger输出Panic级别日志并触发panic
func Panic(msg string, fields ...Field) {
	std.Panic(msg, fields...)
}

// Fatal 使用默认Logger输出Fatal级别日志并调用os.Exit(1)
func Fatal(msg string, fields ...Field) {
	std.Fatal(msg, fields...)
}

// With 使用默认Logger创建带有字段的新Logger
func With(fields ...Field) Logger {
	return std.With(fields...)
}

// SetLevel 设置默认Logger的日志级别
func SetLevel(level Level) {
	std.SetLevel(level)
}

// SetDefault 设置默认Logger
func SetDefault(logger Logger) {
	mu.Lock()
	defer mu.Unlock()
	std = logger
}

// DefaultLogger 返回默认Logger
func DefaultLogger() Logger {
	mu.RLock()
	defer mu.RUnlock()
	return std
}
