package model

import (
	"encoding/json"
	"strings"

	"gorm.io/datatypes"
)

// ModelCapabilities 描述模型或端点支持的任务类型。
type ModelCapabilities struct {
	TextChat      bool `json:"textChat,omitempty"`
	ImageGenerate bool `json:"imageGenerate,omitempty"`
	ImageEdit     bool `json:"imageEdit,omitempty"`
}

// Supports 返回当前能力集是否支持指定任务类型。
// 为兼容旧数据，空配置默认视为仅支持文本对话。
func (c ModelCapabilities) Supports(taskType GenerationTaskType) bool {
	switch taskType {
	case GenerationTaskTypeImageGenerate:
		return c.ImageGenerate
	case GenerationTaskTypeImageEdit:
		return c.ImageEdit
	case GenerationTaskTypeTextChat, "":
		if !c.TextChat && !c.ImageGenerate && !c.ImageEdit {
			return true
		}
		return c.TextChat
	default:
		return false
	}
}

// ModelBaseConfig 是 BaseConfig 的推荐结构。
type ModelBaseConfig struct {
	Provider        string             `json:"provider,omitempty"`
	Route           string             `json:"route,omitempty"`
	DefaultTaskType GenerationTaskType `json:"defaultTaskType,omitempty"`
	Capabilities    ModelCapabilities  `json:"capabilities,omitempty"`
	Meta            map[string]any     `json:"meta,omitempty"`
}

// ModelLimitsConfig 是 Limits 的推荐结构。
type ModelLimitsConfig struct {
	MaxContextTokens int      `json:"maxContextTokens,omitempty"`
	MaxOutputTokens  int      `json:"maxOutputTokens,omitempty"`
	MaxImageCount    int      `json:"maxImageCount,omitempty"`
	ImageSizes       []string `json:"imageSizes,omitempty"`
}

func ParseModelBaseConfig(raw datatypes.JSON) (ModelBaseConfig, error) {
	var cfg ModelBaseConfig
	if len(raw) == 0 {
		cfg.Capabilities.TextChat = true
		return cfg, nil
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ModelBaseConfig{}, err
	}
	if !cfg.Capabilities.TextChat && !cfg.Capabilities.ImageGenerate && !cfg.Capabilities.ImageEdit {
		cfg.Capabilities.TextChat = true
	}
	if !cfg.DefaultTaskType.IsValid() {
		cfg.DefaultTaskType = ""
	}
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	cfg.Route = strings.TrimSpace(cfg.Route)
	return cfg, nil
}

func ParseModelLimitsConfig(raw datatypes.JSON) (ModelLimitsConfig, error) {
	var cfg ModelLimitsConfig
	if len(raw) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ModelLimitsConfig{}, err
	}
	return cfg, nil
}

func MustJSON(v any) datatypes.JSON {
	if v == nil {
		return nil
	}
	bs, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return datatypes.JSON(bs)
}

func (m *GblModel) ParseBaseConfig() (ModelBaseConfig, error) {
	if m == nil {
		return ParseModelBaseConfig(nil)
	}
	return ParseModelBaseConfig(m.BaseConfig)
}

func (m *GblModel) ParseLimitsConfig() (ModelLimitsConfig, error) {
	if m == nil {
		return ModelLimitsConfig{}, nil
	}
	return ParseModelLimitsConfig(m.Limits)
}

func (m *GblModel) SupportsTaskType(taskType GenerationTaskType) bool {
	cfg, err := m.ParseBaseConfig()
	if err != nil {
		return false
	}
	return cfg.Capabilities.Supports(taskType)
}

func (e *GblEndpoint) ParseBaseConfig() (ModelBaseConfig, error) {
	if e == nil {
		return ParseModelBaseConfig(nil)
	}
	return ParseModelBaseConfig(e.BaseConfig)
}

func (e *GblEndpoint) ParseLimitsConfig() (ModelLimitsConfig, error) {
	if e == nil {
		return ModelLimitsConfig{}, nil
	}
	return ParseModelLimitsConfig(e.Limits)
}

func (e *GblEndpoint) SupportsTaskType(taskType GenerationTaskType) bool {
	cfg, err := e.ParseBaseConfig()
	if err != nil {
		return false
	}
	return cfg.Capabilities.Supports(taskType)
}
