package model

import (
	"gorm.io/gorm"
	"time"
)

type GormModel struct {
	ID        uint      `gorm:"primaryKey;autoIncrement;index" json:"id,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty" gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `json:"updatedAt,omitempty" gorm:"column:updated_at;type:datetime;autoUpdateTime"`
}

type GormAuditModel struct {
	CreatedBy string `gorm:"type:varchar(64);column:created_by" json:"createdBy"`
	UpdatedBy string `gorm:"type:varchar(64);column:updated_by" json:"updatedBy"`
}

type GormDeleteModel struct {
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type GormTenantModel struct {
	TenantId string `json:"tenant_id,omitempty"`
}
