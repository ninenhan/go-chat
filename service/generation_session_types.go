package service

import (
	"context"

	"github.com/ninenhan/go-chat/core"
	"github.com/ninenhan/go-chat/model"
)

// GenerationChatCaller 定义底层模型调用函数，便于替换供应商实现或测试 mock。
type GenerationChatCaller func(ctx context.Context, xReq *core.XRequest) (chan any, error)

// GenerationImageCaller 定义图片任务调用函数，便于替换不同供应商实现。
type GenerationImageCaller func(ctx context.Context, req *model.GenerationImageRequest) (*model.GenerationImageResponse, error)

// GenerationTaskResolver 定义会话轮次路由函数。
// 优先推荐上游显式传 turnType；resolver 作为兜底策略。
type GenerationTaskResolver func(ctx context.Context, session *model.GenerationSession, req *model.GenerationSessionChatRequest) model.GenerationTaskType

// GenerationSummarizeRequest 是上下文压缩时的输入。
type GenerationSummarizeRequest struct {
	SessionID       string
	ExistingSummary string
	Messages        []model.GenerationMessage
	LimitTokens     int
}

// GenerationSummarizer 定义会话超上下文时的总结器。
type GenerationSummarizer func(ctx context.Context, req GenerationSummarizeRequest) (string, error)

// GenerationSessionStore 定义会话持久化抽象。
type GenerationSessionStore interface {
	SaveSession(ctx context.Context, session *model.GenerationSession) error
	GetSession(ctx context.Context, sessionID string) (*model.GenerationSession, error)
	SaveSessionMessage(ctx context.Context, sessionID string, message *model.GenerationMessage) error
	UpdateSessionSummary(ctx context.Context, sessionID, summary string) error
}
