package service

import (
	"context"
	"errors"
	"github.com/ninenhan/go-chat/config"
	"github.com/ninenhan/go-chat/model"
	"gorm.io/gorm"
	"log"
	"log/slog"
	"strings"
)

type ModelService struct {
	orm   *gorm.DB
	table string
}

func NewModelService(db *gorm.DB) *ModelService {
	return NewModelServiceWithTable(db, "")
}

func NewModelServiceWithTable(db *gorm.DB, table string) *ModelService {
	if table == "" {
		table = (&model.GblModel{}).TableName()
	}
	if err := db.Table(table).AutoMigrate(&model.GblModel{}); err != nil {
		log.Fatalf("初始化模型表失败: %v", err)
	}
	return &ModelService{orm: db, table: table}
}

func (s *ModelService) q() *gorm.DB {
	return s.orm.Table(s.table)
}

func (s *ModelService) CreateModel(item *model.GblModel) (*model.GblModel, error) {
	if item == nil {
		return nil, errors.New("model 不能为空")
	}
	if err := validateModelPayload(item); err != nil {
		return nil, err
	}
	if err := s.q().Create(item).Error; err != nil {
		kind := ClassifierMySQLError(err)
		return nil, errors.New(kind.Label())
	}
	return item, nil
}

func (s *ModelService) GetModelByID(id any) (*model.GblModel, error) {
	if id == nil {
		return nil, errors.New("model ID 不能为空")
	}
	var item model.GblModel
	if err := s.q().First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *ModelService) GetModelByCode(ctx context.Context, code string) (*model.GblModel, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("model code 不能为空")
	}
	var item model.GblModel
	if err := s.q().WithContext(ctx).Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *ModelService) DeleteModel(id any) error {
	if id == nil {
		return errors.New("model ID 不能为空")
	}
	return s.q().Delete(&model.GblModel{}, id).Error
}

func (s *ModelService) QueryModels(pageRo *model.PageRO, apply ...func(*gorm.DB) *gorm.DB) (*model.PageVO[model.GblModel], error) {
	if pageRo == nil {
		pageRo = &model.PageRO{PageNum: 1, PageSize: 20}
	}
	var list []model.GblModel
	db := s.q().Model(&model.GblModel{}).Order("id DESC")
	vo, err := Paginate[model.GblModel](
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

func (s *ModelService) UpdateModel(item *model.GblModel) (*model.GblModel, error) {
	if item == nil || item.ID == 0 {
		return nil, errors.New("model ID 不能为空")
	}
	var existing model.GblModel
	if err := s.q().First(&existing, item.ID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.Name) == "" {
		item.Name = existing.Name
	}
	if strings.TrimSpace(item.Code) == "" {
		item.Code = existing.Code
	}
	if item.Remark == "" && existing.Remark != "" {
		item.Remark = existing.Remark
	}
	if len(item.BaseConfig) == 0 && len(existing.BaseConfig) > 0 {
		item.BaseConfig = existing.BaseConfig
	}
	if len(item.Limits) == 0 && len(existing.Limits) > 0 {
		item.Limits = existing.Limits
	}
	if item.CreatedBy == "" {
		item.CreatedBy = existing.CreatedBy
	}
	if err := validateModelPayload(item); err != nil {
		return nil, err
	}

	updatePayload := map[string]any{
		"name":        item.Name,
		"code":        item.Code,
		"base_config": item.BaseConfig,
		"limits":      item.Limits,
		"remark":      item.Remark,
	}
	if item.UpdatedBy != "" {
		updatePayload["updated_by"] = item.UpdatedBy
	}
	if err := s.q().Model(&model.GblModel{}).
		Where("id = ?", item.ID).
		Updates(updatePayload).Error; err != nil {
		slog.Error("更新模型失败", "id", item.ID, "err", err)
		return nil, err
	}
	var updated model.GblModel
	if err := s.q().First(&updated, item.ID).Error; err != nil {
		return nil, err
	}
	return &updated, nil
}

func (s *ModelService) GetModelByCodeAndTaskType(ctx context.Context, code string, taskType model.GenerationTaskType) (*model.GblModel, error) {
	item, err := s.GetModelByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if taskType.IsValid() && !item.SupportsTaskType(taskType) {
		return nil, errors.New("model 不支持指定任务类型")
	}
	return item, nil
}

func validateModelPayload(item *model.GblModel) error {
	if strings.TrimSpace(item.Code) == "" {
		return errors.New("model code 不能为空")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("model name 不能为空")
	}
	return nil
}
