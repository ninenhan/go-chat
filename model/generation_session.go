package model

import (
	"feo.vip/chat/core"
	"time"
)

// GenerationMessageRole 定义会话消息角色。
type GenerationMessageRole string

const (
	GenerationRoleSystem    GenerationMessageRole = "system"
	GenerationRoleUser      GenerationMessageRole = "user"
	GenerationRoleAssistant GenerationMessageRole = "assistant"
)

// GenerationMessageFeedback 定义消息反馈标记。
type GenerationMessageFeedback string

const (
	GenerationFeedbackLike    GenerationMessageFeedback = "like"
	GenerationFeedbackDislike GenerationMessageFeedback = "dislike"
)

// GenerationMessage 是会话中的单条消息。
type GenerationMessage struct {
	MessageID           string                     `json:"messageId"`
	Role                GenerationMessageRole      `json:"role"`
	Content             string                     `json:"content"`
	Feedback            *GenerationMessageFeedback `json:"feedback,omitempty"`
	LLMResponseID       string                     `json:"llmResponseId,omitempty"`
	Usage               *core.Usage                `json:"usage,omitempty"`
	LLMContent          string                     `json:"-"`
	PromptSource        string                     `json:"-"`
	PromptTemplateID    uint                       `json:"-"`
	PromptTemplateCode  string                     `json:"-"`
	PromptDisplayMode   string                     `json:"-"`
	ReferenceMessageIDs []string                   `json:"referenceMessageIds,omitempty"`
	Tokens              int                        `json:"tokens,omitempty"`
	CreatedAt           time.Time                  `json:"createdAt"`
}

type GenerationTemplateSlot struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// GenerationSessionInjectedMemory 描述本轮实际注入到上下文的记忆片段（用于 trace）。
type GenerationSessionInjectedMemory struct {
	Kind    string `json:"kind"`
	Scope   string `json:"scope,omitempty"`
	Tokens  int    `json:"tokens,omitempty"`
	Content string `json:"content,omitempty"`
}

// GenerationSessionMemoryTrace 记录本轮记忆注入决策与结果（用于调试/灰度）。
type GenerationSessionMemoryTrace struct {
	Enabled        bool                              `json:"enabled"`
	Mode           string                            `json:"mode"`
	BudgetTokens   int                               `json:"budgetTokens,omitempty"`
	ConflictPolicy string                            `json:"conflictPolicy,omitempty"`
	Conflict       bool                              `json:"conflict,omitempty"`
	ConflictReason string                            `json:"conflictReason,omitempty"`
	Injected       []GenerationSessionInjectedMemory `json:"injected,omitempty"`
}

// GenerationSessionStatus 定义会话状态。
type GenerationSessionStatus string

const (
	GenerationSessionActive GenerationSessionStatus = "ACTIVE"
	GenerationSessionClosed GenerationSessionStatus = "CLOSED"
)

// GenerationSession 是会话聚合对象。每个任务对应一个会话。
type GenerationSession struct {
	SessionID            string                  `json:"sessionId"`
	TaskID               string                  `json:"taskId"`
	Model                string                  `json:"model"`
	LatestMessageID      string                  `json:"latestMessageId,omitempty"`
	Endpoint             *core.EndpointSelector  `json:"-"`
	XRequest             *core.XRequest          `json:"-"`
	Status               GenerationSessionStatus `json:"status"`
	SystemPrompts        []string                `json:"systemPrompts,omitempty"`
	Summary              string                  `json:"summary,omitempty"`
	ContextLimitTokens   int                     `json:"contextLimitTokens,omitempty"`
	ReservedOutputTokens int                     `json:"reservedOutputTokens,omitempty"`
	Messages             []GenerationMessage     `json:"messages,omitempty"`
	CreatedAt            time.Time               `json:"createdAt"`
	UpdatedAt            time.Time               `json:"updatedAt"`
}

// GenerationSessionStartRequest 用于创建会话。
type GenerationSessionStartRequest struct {
	SessionID            string                 `json:"sessionId,omitempty"`
	Model                string                 `json:"model"`
	TemplateID           uint                   `json:"templateId,omitempty"`
	TemplateCode         string                 `json:"templateCode,omitempty"`
	TemplateVars         map[string]string      `json:"templateVars,omitempty"`
	SystemPrompts        []string               `json:"systemPrompts,omitempty"`
	ContextLimitTokens   int                    `json:"contextLimitTokens,omitempty"`
	ReservedOutputTokens int                    `json:"reservedOutputTokens,omitempty"`
	Endpoint             *core.EndpointSelector `json:"endpoint,omitempty"`
	XRequest             *core.XRequest         `json:"xRequest,omitempty"`
}

// GenerationSessionChatRequest 用于会话多轮对话。
type GenerationSessionChatRequest struct {
	SessionID string `json:"sessionId"`

	BaseURL string `json:"baseURL"`
	APIKey  string `json:"apiKey"`
	Model   string `json:"model,omitempty"`

	Prompt            string                   `json:"prompt"`
	Template          string                   `json:"template"`
	TemplateVars      map[string]string        `json:"templateVars"`
	TemplateID        uint                     `json:"templateId,omitempty"`
	TemplateSlots     []GenerationTemplateSlot `json:"templateSlots,omitempty"`
	DisplayPrompt     string                   `json:"displayPrompt,omitempty"`
	PromptDisplayMode string                   `json:"promptDisplayMode,omitempty"`

	SystemPrompts        []string            `json:"systemPrompts,omitempty"`
	ReferenceMessageIDs  []string            `json:"referenceMessageIds,omitempty"`
	MemoryEnabled        *bool               `json:"memoryEnabled,omitempty"`
	MemoryMode           string              `json:"memoryMode,omitempty"`
	MemoryBudgetTokens   int                 `json:"memoryBudgetTokens,omitempty"`
	MemoryConflictPolicy string              `json:"memoryConflictPolicy,omitempty"`
	MemoryTrace          bool                `json:"memoryTrace,omitempty"`
	ContextLimitTokens   int                 `json:"contextLimitTokens,omitempty"`
	ReservedOutputTokens int                 `json:"reservedOutputTokens,omitempty"`
	Stream               bool                `json:"stream"`
	ExtraHeaders         map[string][]string `json:"extraHeaders"`
	ExtraBody            map[string]any      `json:"extraBody"`
	BaseGenerationRequest
}

// GenerationSessionChatResponse 是会话轮次响应。
type GenerationSessionChatResponse struct {
	SessionID          string                        `json:"sessionId"`
	TaskID             string                        `json:"taskId"`
	UserMessageID      string                        `json:"userMessageId"`
	AssistantMessageID string                        `json:"assistantMessageId"`
	Prompt             string                        `json:"prompt"`
	Output             string                        `json:"output"`
	Chunks             []string                      `json:"chunks,omitempty"`
	Raw                any                           `json:"raw,omitempty"`
	Status             GenerationTaskStatus          `json:"status,omitempty"`
	UsedSummary        bool                          `json:"usedSummary"`
	Summary            string                        `json:"summary,omitempty"`
	ContextTokens      int                           `json:"contextTokens,omitempty"`
	MemoryTrace        *GenerationSessionMemoryTrace `json:"memoryTrace,omitempty"`
}
