package model

import (
	"encoding/json"
	"feo.vip/chat/core"
	"fmt"
	"gorm.io/datatypes"
	"time"
)

type GblEndpoint struct {
	ID                 uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	Name               string         `gorm:"column:name;type:varchar(128);not null;uniqueIndex:udx_endpoint"`
	Description        string         `gorm:"column:description;type:varchar(255)"`
	Type               string         `gorm:"column:type;type:varchar(32);not null;default:'HTTP'"`
	Enabled            bool           `gorm:"column:enabled;type:tinyint(1);not null;default:1;index" json:"enabled"`
	Payload            datatypes.JSON `gorm:"column:payload;type:json"`
	CreatedAt          time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	ContextLimitTokens int            `gorm:"column:context_limit_tokens"`
	ModelCode          string         `gorm:"column:model_code;type:varchar(128);index"`
	BaseConfig         datatypes.JSON `gorm:"column:base_config;type:json"`
	Limits             datatypes.JSON `gorm:"column:limits;type:json"`
	Label              string         `gorm:"column:label;type:varchar(64)"`
}

func (*GblEndpoint) TableName() string {
	return "gbl_endpoints"
}

func (e *GblEndpoint) ToHttpEndpoint() (*core.HttpEndpoint, error) {
	endpoint := &core.HttpEndpoint{
		Endpoint: core.Endpoint{
			Id:          fmt.Sprintf("%d", e.ID),
			Name:        e.Name,
			Description: e.Description,
			Type:        core.EndpointType(e.Type),
		},
	}
	if len(e.Payload) > 0 {
		var payload core.HttpEndpointPayload
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			return nil, err
		}
		endpoint.Payload = &payload
	}
	return endpoint, nil
}
