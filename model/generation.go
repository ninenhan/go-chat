package model

import "github.com/ninenhan/go-chat/core"

// GenerationSlot 描述模板中的一个插槽。
type GenerationSlot struct {
	Key         string `json:"key"`
	Placeholder string `json:"placeholder"`
}

type BaseGenerationRequest struct {
	Model    string                 `json:"model"`
	Endpoint *core.EndpointSelector `json:"endpoint,omitempty"`
	XRequest *core.XRequest         `json:"xRequest,omitempty"`
}

// GenerationGenerateRequest 是 AI 文案生成的输入参数。
type GenerationGenerateRequest struct {
	TaskID string `json:"taskId,omitempty"`

	BaseURL string `json:"baseURL"`
	APIKey  string `json:"apiKey"`

	Prompt       string            `json:"prompt"`
	Template     string            `json:"template"`
	TemplateVars map[string]string `json:"templateVars"`

	SystemPrompts []string            `json:"systemPrompts"`
	ExtraHeaders  map[string][]string `json:"extraHeaders"`
	ExtraBody     map[string]any      `json:"extraBody"`
	BaseGenerationRequest
}

// GenerationTaskStatus 描述生成过程状态（用于响应流转语义）。
type GenerationTaskStatus string

const (
	GenerationTaskQueued    GenerationTaskStatus = "QUEUED"
	GenerationTaskRunning   GenerationTaskStatus = "RUNNING"
	GenerationTaskCompleted GenerationTaskStatus = "COMPLETED"
	GenerationTaskFailed    GenerationTaskStatus = "FAILED"
)

// GenerationGenerateResponse 是 AI 文案生成结果。
type GenerationGenerateResponse struct {
	TaskID string               `json:"taskId,omitempty"`
	Status GenerationTaskStatus `json:"status,omitempty"`
	Prompt string               `json:"prompt"`
	Output string               `json:"output"`
	Chunks []string             `json:"chunks,omitempty"`
	Raw    any                  `json:"raw,omitempty"`
}
