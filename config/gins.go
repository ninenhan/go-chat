package config

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"runtime/debug"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Result  interface{} `json:"result,omitempty"`
}

// AppError 自定义错误类型
type AppError struct {
	Code    int    `json:"code"`    // 错误码
	Message string `json:"message"` // 错误信息
}

func NewAppSuccessResponse(data interface{}) *Response {
	return &Response{
		Code:    0,
		Message: "",
		Result:  data,
	}
}

// 实现 `error` 接口
func (e *AppError) Error() string {
	return e.Message
}

// NewAppError 创建新的 AppError
func NewAppError(code int, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// 判断是否为 WebSocket 请求
func isWebSocket(c *gin.Context) bool {
	upgrade := c.GetHeader("Connection")
	websocket := c.GetHeader("Upgrade")
	return upgrade == "upgrade" && websocket == "websocket"
}

// 判断是否为 SSE 请求
func isSSE(c *gin.Context) bool {
	return c.GetHeader("Accept") == "text/event-stream"
}

func GinErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isWebSocket(c) || isSSE(c) {
			c.Next()
			return
		}
		defer func() {
			if r := recover(); r != nil {
				// 捕获 panic 异常并获取异常信息
				var errMsg string
				switch v := r.(type) {
				case string:
					// 如果 panic 是一个字符串
					errMsg = v
				case error:
					// 如果 panic 是一个错误
					errMsg = v.Error()
				default:
					// 如果 panic 是其他类型
					errMsg = fmt.Sprintf("%v", v)
				}
				slog.Error("GinError", "error", r, "stack", errMsg)
				fmt.Println(string(debug.Stack()))
				// 捕获 panic 异常（可以是未知错误）
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    http.StatusInternalServerError,
					"message": "系统异常",
				})
				c.Abort()
			}
		}()
		c.Next() // 执行其他中间件或业务逻辑
		if len(c.Errors) > 0 {
			// 获取最后一个错误
			lastErr := c.Errors.Last()
			var appErr *AppError
			if errors.As(lastErr.Err, &appErr) {
				// 如果是 AppError，返回标准化错误响应
				c.JSON(appErr.Code, gin.H{
					"code":    appErr.Code,
					"message": appErr.Message,
				})
			}
			c.Abort()
			return
		}
	}
}

func AddAppError(c *gin.Context, error *AppError) {
	_ = c.Error(error)
}

func ResponseOk(c *gin.Context, data any) {
	c.JSON(http.StatusOK, *NewAppSuccessResponse(data))
}

func ResponseOkMessage(c *gin.Context, message string) {
	c.JSON(http.StatusOK, &Response{
		Code:    0,
		Message: message,
		Result:  nil,
	})
}

func ResponseXhrError(c *gin.Context, message string) {
	c.JSON(http.StatusOK, *NewAppError(1, message))
}
