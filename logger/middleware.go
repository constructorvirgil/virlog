package logger

import (
	"context"
	"net/http"
	"time"
)

// 定义上下文key类型，用于从上下文提取日志字段
type loggerContextKey struct{}

// HTTPMiddleware 返回一个用于HTTP服务的日志中间件
func HTTPMiddleware(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 创建请求ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}

			// 将请求ID添加到响应头
			w.Header().Set("X-Request-ID", requestID)

			// 创建响应记录器
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				responseSize:   0,
			}

			// 创建请求上下文的logger
			reqLogger := logger.With(
				String("request_id", requestID),
				String("method", r.Method),
				String("path", r.URL.Path),
				String("remote_addr", r.RemoteAddr),
				String("user_agent", r.UserAgent()),
			)

			// 将logger添加到上下文
			ctx := context.WithValue(r.Context(), loggerContextKey{}, reqLogger)

			// 请求开始日志
			reqLogger.Info("HTTP request started")

			// 处理请求
			next.ServeHTTP(rw, r.WithContext(ctx))

			// 计算请求处理时间
			duration := time.Since(start)

			// 请求结束日志
			reqLogger.Info("HTTP request completed",
				Int("status", rw.statusCode),
				Int64("bytes", rw.responseSize),
				Duration("latency", duration),
			)
		})
	}
}

// GetLoggerFromContext 从HTTP请求上下文中获取Logger
func GetLoggerFromContext(ctx context.Context) Logger {
	if ctx == nil {
		return DefaultLogger()
	}
	if ctxLogger, ok := ctx.Value(loggerContextKey{}).(Logger); ok {
		return ctxLogger
	}
	return DefaultLogger()
}

// responseWriter 是对http.ResponseWriter的封装，用于捕获状态码和响应大小
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
}

// WriteHeader 实现http.ResponseWriter接口
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write 实现http.ResponseWriter接口
func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.responseSize += int64(size)
	return size, err
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	// 简单实现，实际项目可能需要更复杂的UUID生成
	return time.Now().Format("20060102150405") + "-" + randString(8)
}

// randString 生成随机字符串
func randString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[time.Now().UnixNano()%int64(len(letterBytes))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
