package service

import (
	"context"
	"errors"
	"feo.vip/chat/config"
	"feo.vip/chat/model"
	"gorm.io/gorm"
	"log"
	"log/slog"
)

type PromptService struct {
	orm   *gorm.DB
	table string
}

func NewPromptService(db *gorm.DB) *PromptService {
	return NewPromptServiceWithTable(db, "")
}

func NewPromptServiceWithTable(db *gorm.DB, table string) *PromptService {
	if table == "" {
		table = (&model.GblPrompt{}).TableName() // 默认表
	}
	if err := db.Table(table).AutoMigrate(&model.GblPrompt{}); err != nil {
		log.Fatalf("初始化Prompt表失败: %v", err)
	}
	return &PromptService{orm: db, table: table}
}

func (s *PromptService) q() *gorm.DB {
	return s.orm.Table(s.table)
}

func (s *PromptService) CreatePrompt(prompt *model.GblPrompt) (*model.GblPrompt, error) {
	if prompt == nil {
		return nil, errors.New("prompt 不能为空")
	}
	if err := validatePromptPayload(prompt); err != nil {
		return nil, err
	}
	if err := s.q().Create(prompt).Error; err != nil {
		kind := ClassifierMySQLError(err)
		return nil, errors.New(kind.Label())
	}
	return prompt, nil
}

func (s *PromptService) GetPromptByID(id any) (*model.GblPrompt, error) {
	if id == nil {
		return nil, errors.New("prompt ID 不能为空")
	}
	var prompt model.GblPrompt
	if err := s.q().First(&prompt, id).Error; err != nil {
		return nil, err
	}
	return &prompt, nil
}

func (s *PromptService) GetPromptByCode(ctx context.Context, code string) (*model.GblPrompt, error) {
	var prompt model.GblPrompt
	if err := s.q().WithContext(ctx).Where("code = ?", code).First(&prompt).Error; err != nil {
		return nil, err
	}
	return &prompt, nil
}

func (s *PromptService) DeletePrompt(id any) error {
	if id == nil {
		return errors.New("prompt ID 不能为空")
	}
	return s.q().Delete(&model.GblPrompt{}, id).Error
}

func (s *PromptService) QueryPrompts(pageRo *model.PageRO, apply ...func(*gorm.DB) *gorm.DB) (*model.PageVO[model.GblPrompt], error) {
	if pageRo == nil {
		pageRo = &model.PageRO{PageNum: 1, PageSize: 20}
	}
	var list []model.GblPrompt
	db := s.q().Model(&model.GblPrompt{}).Order("id DESC")
	vo, err := Paginate[model.GblPrompt](
		db,
		pageRo,
		&list,
		append([]func(*gorm.DB) *gorm.DB{
			func(db *gorm.DB) *gorm.DB {
				if len(pageRo.Condition) > 0 {
					db = config.ApplyConditions(db, pageRo.Condition)
				}
				if len(pageRo.FilterCondition) > 0 {
					db = config.ApplyConditions(db, pageRo.FilterCondition)
				}
				if len(pageRo.Sorts) > 0 {
					db = config.ApplySorts(db, pageRo.Sorts)
				}
				return db
			},
		}, apply...)...,
	)
	return vo, err
}

func (s *PromptService) UpdatePrompt(prompt *model.GblPrompt) (*model.GblPrompt, error) {
	if prompt == nil || prompt.ID == 0 {
		return nil, errors.New("prompt ID 不能为空")
	}
	var existing model.GblPrompt
	if err := s.q().First(&existing, prompt.ID).Error; err != nil {
		return nil, err
	}
	// 缺省字段沿用已有值，避免更新时被意外重置。
	if prompt.Name == "" {
		prompt.Name = existing.Name
	}
	if prompt.Code == "" {
		prompt.Code = existing.Code
	}
	if prompt.Description == "" && existing.Description != "" {
		prompt.Description = existing.Description
	}
	if prompt.Content == "" {
		prompt.Content = existing.Content
	}
	if prompt.SystemPrompt == "" && existing.SystemPrompt != "" {
		prompt.SystemPrompt = existing.SystemPrompt
	}
	if len(prompt.Variables) == 0 && len(existing.Variables) > 0 {
		prompt.Variables = existing.Variables
	}
	if prompt.Scope == "" {
		prompt.Scope = existing.Scope
	}
	if prompt.TenantId == "" {
		prompt.TenantId = existing.TenantId
	}
	if prompt.CreatedBy == "" {
		prompt.CreatedBy = existing.CreatedBy
	}
	if err := validatePromptPayload(prompt); err != nil {
		return nil, err
	}
	updatePayload := map[string]any{
		"code":          prompt.Code,
		"name":          prompt.Name,
		"description":   prompt.Description,
		"content":       prompt.Content,
		"system_prompt": prompt.SystemPrompt,
		"variables":     prompt.Variables,
		"scope":         prompt.Scope,
		"tenant_id":     prompt.TenantId,
	}
	if prompt.UpdatedBy != "" {
		updatePayload["updated_by"] = prompt.UpdatedBy
	}
	if err := s.q().Model(&model.GblPrompt{}).
		Where("id = ?", prompt.ID).
		Updates(updatePayload).Error; err != nil {
		slog.Error("更新提示词失败", "id", prompt.ID, "err", err)
		return nil, err
	}
	var updated model.GblPrompt
	if err := s.q().First(&updated, prompt.ID).Error; err != nil {
		return nil, err
	}
	return &updated, nil
}

func validatePromptPayload(prompt *model.GblPrompt) error {
	if prompt.Code == "" {
		return errors.New("prompt 的 code 不能为空")
	}
	if prompt.Name == "" || prompt.Content == "" {
		return errors.New("prompt 的名称与内容不能为空")
	}
	if prompt.Scope == "" {
		prompt.Scope = model.PromptScopeAllTenants
	}
	if !prompt.Scope.IsValid() {
		return errors.New("prompt scope 非法")
	}
	if prompt.Scope != model.PromptScopeAllTenants && prompt.TenantId == "" {
		return errors.New("tenant scope 需要 tenantId")
	}
	if prompt.Scope == model.PromptScopeCreator && prompt.CreatedBy == "" {
		return errors.New("仅创建者可见的 prompt 需要 createdBy")
	}
	return nil
}
