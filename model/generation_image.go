package model

// GenerationAttachment 描述一次会话输入中携带的媒体对象。
type GenerationAttachment struct {
	ID       string `json:"id,omitempty"`
	Kind     string `json:"kind,omitempty"`
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Name     string `json:"name,omitempty"`
}

// GenerationArtifact 描述模型产出的媒体对象。
type GenerationArtifact struct {
	ID       string `json:"id,omitempty"`
	Kind     string `json:"kind,omitempty"`
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Name     string `json:"name,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Seed     int64  `json:"seed,omitempty"`
}

// GenerationImageRequest 是文生图/图生图专用请求。
type GenerationImageRequest struct {
	TaskID         string                 `json:"taskId,omitempty"`
	TaskType       GenerationTaskType     `json:"taskType,omitempty"`
	BaseURL        string                 `json:"baseURL"`
	APIKey         string                 `json:"apiKey"`
	Model          string                 `json:"model"`
	Prompt         string                 `json:"prompt"`
	NegativePrompt string                 `json:"negativePrompt,omitempty"`
	InputImages    []GenerationAttachment `json:"inputImages,omitempty"`
	Image          string                 `json:"image,omitempty"`
	Images         []string               `json:"images,omitempty"`
	MaskImage      *GenerationAttachment  `json:"maskImage,omitempty"`
	Size           string                 `json:"size,omitempty"`
	Quality        string                 `json:"quality,omitempty"`
	Style          string                 `json:"style,omitempty"`
	N              int                    `json:"n,omitempty"`
	OutputFormat   string                 `json:"output_format,omitempty"`
	Watermark      *bool                  `json:"watermark,omitempty"`
	ExtraHeaders   map[string][]string    `json:"extraHeaders,omitempty"`
	ExtraBody      map[string]any         `json:"extraBody,omitempty"`
	BaseGenerationRequest
}

// GenerationImageResponse 是图片任务响应。
type GenerationImageResponse struct {
	TaskID    string               `json:"taskId,omitempty"`
	TaskType  GenerationTaskType   `json:"taskType,omitempty"`
	Status    GenerationTaskStatus `json:"status,omitempty"`
	Prompt    string               `json:"prompt"`
	Output    string               `json:"output,omitempty"`
	Artifacts []GenerationArtifact `json:"artifacts,omitempty"`
	Raw       any                  `json:"raw,omitempty"`
}
