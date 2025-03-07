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
	GetZapLogger() *zap.Logger
}

// 确保 zapLogger 实现了 Logger 接口
var _ Logger = (*zapLogger)(nil)

// zapLogger 是对 zap.Logger 的封装
type zapLogger struct {
	rawZapLogger *zap.Logger
	atom         *zap.AtomicLevel
	config       *config.Config
	fields       []Field
	mu           sync.RWMutex
}

// NewLogger 创建一个新的Logger实例
func NewLogger(cfg *config.Config) (Logger, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	atom := zap.NewAtomicLevel()
	// 设置日志级别
	switch cfg.Level {
	case "debug":
		atom.SetLevel(DebugLevel)
	case "info":
		atom.SetLevel(InfoLevel)
	case "warn":
		atom.SetLevel(WarnLevel)
	case "error":
		atom.SetLevel(ErrorLevel)
	case "dpanic":
		atom.SetLevel(DPanicLevel)
	case "panic":
		atom.SetLevel(PanicLevel)
	case "fatal":
		atom.SetLevel(FatalLevel)
	default:
		atom.SetLevel(InfoLevel)
	}

	// 初始化核心配置
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

	// 配置输出
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

	// 配置编码器
	var encoder zapcore.Encoder
	if cfg.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// 创建Core
	core := zapcore.NewCore(encoder, writeSyncer, atom)

	// 添加默认字段
	fields := make([]zap.Field, 0, len(cfg.DefaultFields))
	for k, v := range cfg.DefaultFields {
		fields = append(fields, zap.Any(k, v))
	}

	// 创建Logger
	rawZapLogger := zap.New(core, getZapOptions(cfg)...).With(fields...)

	return &zapLogger{
		rawZapLogger: rawZapLogger,
		atom:         &atom,
		config:       cfg,
		fields:       make([]Field, 0),
	}, nil
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
func (l *zapLogger) GetZapLogger() *zap.Logger {
	return l.rawZapLogger
}

// 全局默认Logger实例
var std Logger

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
	std = logger
}

// DefaultLogger 返回默认Logger
func DefaultLogger() Logger {
	return std
}
