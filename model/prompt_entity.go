package model

import (
	"encoding/json"
	"github.com/ninenhan/go-workflow/fn"
	"gorm.io/datatypes"
	"log/slog"
)

type PromptScope string

const (
	PromptScopeAllTenants PromptScope = "GLOBAL"
	PromptScopeTenantOnly PromptScope = "TENANT"
	PromptScopeCreator    PromptScope = "CREATOR"
)

func (s PromptScope) IsValid() bool {
	switch s {
	case PromptScopeAllTenants, PromptScopeTenantOnly, PromptScopeCreator:
		return true
	default:
		return false
	}
}

type GblRole string

const (
	GblRole_System    = "system"
	GblRole_Assistant = "assistant"
	GblRole_User      = "user"
)

type GblPromptVariable struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Example  string `json:"example"`
	Required bool   `json:"required"`
}

type GblMessage struct {
	Role    GblRole `json:"role,omitempty"`
	Content string  `json:"content,omitempty"`
}
type GblPrompt struct {
	GormModel
	GormAuditModel
	TenantId string `gorm:"column:tenant_id;type:varchar(64);index" json:"tenantId,omitempty"`
	Code     string `gorm:"column:code;type:varchar(128);not null;uniqueIndex:udx_prompt_code" json:"code"`
	Name     string `gorm:"column:name;type:varchar(128);not null;uniqueIndex:udx_prompt_name" json:"name"`
	// Scope 控制提示词可见性：全租户、租户内、创建者
	Scope        PromptScope    `gorm:"column:scope;type:varchar(16);not null;default:'GLOBAL'" json:"scope"`
	Description  string         `gorm:"column:description;type:varchar(255)" json:"description"`
	Content      string         `gorm:"column:content;type:text;not null" json:"content"`
	SystemPrompt string         `gorm:"column:system_prompt;type:text" json:"systemPrompt"`
	Variables    datatypes.JSON `gorm:"column:variables;type:json" json:"variables"`

	VariableModel []GblPromptVariable `gorm:"-" json:"-"`
	Rendered      []GblMessage        `gorm:"-" json:"rendered"`
}

func (p *GblPrompt) ParseVariable() {
	var variables []GblPromptVariable
	if err := json.Unmarshal(p.Variables, &variables); err != nil {
		slog.Error("PromptVariable Unmarshal error", "err", err)
		return
	}
	p.VariableModel = variables
}

func (p *GblPrompt) RenderPrompt() {
	p.ParseVariable()
	if p.VariableModel == nil {
		return
	}
	varMap := fn.CollectToMap(p.VariableModel, func(t GblPromptVariable) string {
		return t.Key
	}, func(t GblPromptVariable) any {
		return t.Example
	})
	slog.Info("MarshalJSON variables", "varMap", varMap)
	var Rendered []GblMessage
	if p.SystemPrompt != "" {
		message, e := fn.RenderTemplateWithControl(p.SystemPrompt, varMap)
		if e == nil {
			Rendered = append(Rendered, GblMessage{
				Role:    GblRole_System,
				Content: message,
			})
		}
	}
	if p.Content != "" {
		message, e := fn.RenderTemplateWithControl(p.Content, varMap)
		if e == nil {
			Rendered = append(Rendered, GblMessage{
				Role:    GblRole_User,
				Content: message,
			})
		}
	}
	p.Rendered = Rendered
}

func (*GblPrompt) TableName() string {
	return "gbl_prompts"
}
