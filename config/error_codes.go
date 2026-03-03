package config

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"sync"
)

// ErrorCode represents a stable numeric identifier that can be shared across services.
type ErrorCode int

// ErrorDescriptor keeps both code and human friendly message together.
type ErrorDescriptor struct {
	Code    ErrorCode
	Message string
}

// registry keeps the global mapping between error codes and their default messages.
var registry = &errorRegistry{
	messages: make(map[ErrorCode]string),
}

type errorRegistry struct {
	mu       sync.RWMutex
	messages map[ErrorCode]string
}

// RegisterError registers the message for a specific code.
// Panics when the same code is registered twice to avoid silent overwrites.
func RegisterError(code ErrorCode, message string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if existing, ok := registry.messages[code]; ok && existing != message {
		panic(fmt.Sprintf("error code %d already registered with message %q", code, existing))
	}
	registry.messages[code] = message
}

// LookupErrorMessage returns the message associated with the code.
func LookupErrorMessage(code ErrorCode) (string, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	msg, ok := registry.messages[code]
	return msg, ok
}

// NewAppErrorByCode converts ErrorCode to *AppError.
func NewAppErrorByCode(code ErrorCode) *AppError {
	if msg, ok := LookupErrorMessage(code); ok {
		return NewAppError(int(code), msg)
	}
	// Keep last resort fallback to avoid nil responses when code is not registered yet.
	return NewAppError(int(code), "未知错误")
}

// ResponseXhrErrorCode is a helper to respond with an error code directly.
func ResponseXhrErrorCode(c *gin.Context, code ErrorCode) {
	c.JSON(http.StatusOK, *NewAppErrorByCode(code))
}

const (
	ErrCodeCommonUnknown          ErrorCode = 10001
	ErrCodeUserLoginRequired      ErrorCode = 12001
	ErrCodeUserTenantMissing      ErrorCode = 12002
	ErrCodeUserInvalidRequestBody ErrorCode = 12101
	ErrCodeUserRoleCodeMissing    ErrorCode = 12201
	ErrCodeTalentifyProjectAbsent ErrorCode = 14001
	ErrCodeTalentifyStatusInvalid ErrorCode = 14002
	ErrCodeTalentifyUploadMissing ErrorCode = 14101
	ErrCodeTalentifyPricingFailed ErrorCode = 14201
	ErrCodeKoltaParamInvalid      ErrorCode = 16001
	ErrCodeKoltaFavoriteMissing   ErrorCode = 16002
	ErrCodeKoltaQueryFailed       ErrorCode = 16003
)

func init() {
	RegisterError(ErrCodeCommonUnknown, "未知错误")
	RegisterError(ErrCodeUserLoginRequired, "请先登录")
	RegisterError(ErrCodeUserTenantMissing, "无法识别租户信息")
	RegisterError(ErrCodeUserInvalidRequestBody, "无效的请求体")
	RegisterError(ErrCodeUserRoleCodeMissing, "角色编码不能为空")
	RegisterError(ErrCodeTalentifyProjectAbsent, "项目不存在")
	RegisterError(ErrCodeTalentifyStatusInvalid, "状态码检查失败")
	RegisterError(ErrCodeTalentifyUploadMissing, "请指定上传文件")
	RegisterError(ErrCodeTalentifyPricingFailed, "价格校验失败")
	RegisterError(ErrCodeKoltaParamInvalid, "Param Invalid")
	RegisterError(ErrCodeKoltaFavoriteMissing, "该收藏夹不存在或您无权操作")
	RegisterError(ErrCodeKoltaQueryFailed, "查询出错")
}
