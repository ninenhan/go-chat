package service

import (
	"context"
	"errors"
	"github.com/ninenhan/go-chat/config"
	"github.com/ninenhan/go-chat/core"
	"github.com/ninenhan/go-chat/model"
	"github.com/ninenhan/go-workflow/fn"
	"gorm.io/gorm"
	"log"
	"log/slog"
	"strings"
)

type EndpointService struct {
	orm   *gorm.DB
	table string
}

func NewEndpointService(db *gorm.DB) *EndpointService {
	return NewEndpointServiceWithTable(db, "")
}

func NewEndpointServiceWithTable(db *gorm.DB, table string) *EndpointService {
	if table == "" {
		table = (&model.GblEndpoint{}).TableName() // 默认表
	}
	if err := db.Table(table).AutoMigrate(&model.GblEndpoint{}); err != nil {
		log.Fatalf("初始化端点表失败: %v", err)
	}
	return &EndpointService{orm: db, table: table}
}

func (s *EndpointService) q() *gorm.DB {
	return s.orm.Table(s.table)
}

func (s *EndpointService) CreateEndpoint(item *model.GblEndpoint) (*model.GblEndpoint, error) {
	if item == nil {
		return nil, errors.New("endpoint 不能为空")
	}
	if err := validateEndpointPayload(item); err != nil {
		return nil, err
	}
	if err := s.q().Create(item).Error; err != nil {
		kind := ClassifierMySQLError(err)
		return nil, errors.New(kind.Label())
	}
	return item, nil
}

func (s *EndpointService) GetEndpointByID(id any) (*model.GblEndpoint, error) {
	if id == nil {
		return nil, errors.New("endpoint ID 不能为空")
	}
	var item model.GblEndpoint
	if err := s.q().First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *EndpointService) GetEndpointByModelCode(ctx context.Context, code string) (*model.GblEndpoint, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("model code 不能为空")
	}
	var item model.GblEndpoint
	if err := s.q().WithContext(ctx).Where("model_code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *EndpointService) GetEndpointByModelCodeAndTaskType(ctx context.Context, code string, taskType model.GenerationTaskType) (*model.GblEndpoint, error) {
	records, err := s.QueryAvailableModelEndpoints(ctx, code, 200)
	if err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	for i := range records {
		item := records[i]
		if item.ModelCode != code {
			continue
		}
		if taskType.IsValid() && !item.SupportsTaskType(taskType) {
			continue
		}
		return &item, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *EndpointService) DeleteEndpoint(id any) error {
	if id == nil {
		return errors.New("endpoint ID 不能为空")
	}
	return s.q().Delete(&model.GblEndpoint{}, id).Error
}

func (s *EndpointService) UpdateEndpoint(item *model.GblEndpoint) (*model.GblEndpoint, error) {
	if item == nil || item.ID == 0 {
		return nil, errors.New("endpoint ID 不能为空")
	}
	var existing model.GblEndpoint
	if err := s.q().First(&existing, item.ID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.Name) == "" {
		item.Name = existing.Name
	}
	if strings.TrimSpace(item.Description) == "" && existing.Description != "" {
		item.Description = existing.Description
	}
	if strings.TrimSpace(item.Type) == "" {
		item.Type = existing.Type
	}
	if len(item.Payload) == 0 && len(existing.Payload) > 0 {
		item.Payload = existing.Payload
	}
	if item.ContextLimitTokens <= 0 {
		item.ContextLimitTokens = existing.ContextLimitTokens
	}
	if strings.TrimSpace(item.ModelCode) == "" {
		item.ModelCode = existing.ModelCode
	}
	if len(item.BaseConfig) == 0 && len(existing.BaseConfig) > 0 {
		item.BaseConfig = existing.BaseConfig
	}
	if len(item.Limits) == 0 && len(existing.Limits) > 0 {
		item.Limits = existing.Limits
	}
	if strings.TrimSpace(item.Label) == "" && existing.Label != "" {
		item.Label = existing.Label
	}
	if err := validateEndpointPayload(item); err != nil {
		return nil, err
	}

	updatePayload := map[string]any{
		"name":                 item.Name,
		"description":          item.Description,
		"type":                 item.Type,
		"enabled":              item.Enabled,
		"payload":              item.Payload,
		"context_limit_tokens": item.ContextLimitTokens,
		"model_code":           item.ModelCode,
		"base_config":          item.BaseConfig,
		"limits":               item.Limits,
		"label":                item.Label,
	}
	if err := s.q().Model(&model.GblEndpoint{}).Where("id = ?", item.ID).Updates(updatePayload).Error; err != nil {
		slog.Error("更新端点失败", "id", item.ID, "err", err)
		return nil, err
	}
	var updated model.GblEndpoint
	if err := s.q().First(&updated, item.ID).Error; err != nil {
		return nil, err
	}
	return &updated, nil
}

func (s *EndpointService) RequestHttpEndpoint(selector *core.EndpointSelector) *core.XRequest {
	if selector == nil {
		return nil
	}
	pageRo := &model.PageRO{
		PageNum:   1,
		PageSize:  1,
		Condition: selector.Selector,
	}
	endpointPage := s.QueryEndpoints(pageRo)
	if endpointPage == nil || len(endpointPage.List) == 0 {
		return nil
	}
	endpoints := fn.StreamMap(endpointPage.List, func(endpoint model.GblEndpoint) *core.HttpEndpoint {
		p, err := endpoint.ToHttpEndpoint()
		if err != nil {
			return nil
		}
		return p
	})
	records := make([]core.HttpEndpoint, 0)
	if len(endpoints) == 0 {
		return nil
	}
	for i := range endpoints {
		p := endpoints[i]
		if p != nil {
			records = append(records, *p)
		}
	}
	endpoint := endpoints[0]
	request, err := fn.ConvertByJSON[any, core.XRequest](endpoint.Payload)
	if err != nil {
		slog.Error("解析端点请求失败", "err", err)
		return nil
	}
	return &request
}

func (s *EndpointService) QueryEndpoints(pageRo *model.PageRO, apply ...func(*gorm.DB) *gorm.DB) *model.PageVO[model.GblEndpoint] {
	if pageRo == nil {
		pageRo = &model.PageRO{PageNum: 1, PageSize: 20}
	}
	var accounts []model.GblEndpoint
	// 构造带 table + order 的 *gorm.DB
	db := s.q().
		Model(&model.GblEndpoint{}).
		Order("id DESC")
	// 把通用分页拿来用，并把 config.ApplyConditions 当作条件函数传进去
	vo, _ := Paginate[model.GblEndpoint](
		db,
		pageRo,
		&accounts,
		append([]func(*gorm.DB) *gorm.DB{
			func(db *gorm.DB) *gorm.DB {
				if len(pageRo.Condition) > 0 {
					db = config.ApplyConditions(db, pageRo.Condition)
				}
				if len(pageRo.Sorts) > 0 {
					db = config.ApplySorts(db, pageRo.Sorts)
				}
				return db
			},
		}, apply...)...,
	)
	return vo
}

func (s *EndpointService) QueryAvailableModelEndpoints(ctx context.Context, keyword string, limit int) ([]model.GblEndpoint, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	keyword = strings.TrimSpace(keyword)

	db := s.q().WithContext(ctx).
		Model(&model.GblEndpoint{}).
		Where("enabled = ? AND model_code <> ''", true).
		Order("id DESC")
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("(name LIKE ? OR model_code LIKE ? OR label LIKE ?)", like, like, like)
	}

	records := make([]model.GblEndpoint, 0)
	if err := db.Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func validateEndpointPayload(item *model.GblEndpoint) error {
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("endpoint name 不能为空")
	}
	if strings.TrimSpace(item.Type) == "" {
		item.Type = core.EndpointType_HTTP
	}
	return nil
}
