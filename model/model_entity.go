package model

import "gorm.io/datatypes"

// GblModel 定义租户无关的模型配置。
type GblModel struct {
	GormModel
	GormAuditModel
	Name       string         `gorm:"column:name;type:varchar(128);not null;uniqueIndex:udx_model_name" json:"name"`
	Code       string         `gorm:"column:code;type:varchar(128);not null;uniqueIndex:udx_model_code" json:"code"`
	BaseConfig datatypes.JSON `gorm:"column:base_config;type:json" json:"baseConfig,omitempty"`
	Limits     datatypes.JSON `gorm:"column:limits;type:json" json:"limits,omitempty"`
	Remark     string         `gorm:"column:remark;type:varchar(255)" json:"remark,omitempty"`
}

func (*GblModel) TableName() string {
	return "gbl_models"
}
