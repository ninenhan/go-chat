package service

import (
	"context"

	"feo.vip/chat/core"
	"feo.vip/chat/model"
)

// GenerationChatCaller 定义底层模型调用函数，便于替换供应商实现或测试 mock。
type GenerationChatCaller func(ctx context.Context, xReq *core.XRequest) (chan any, error)

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
