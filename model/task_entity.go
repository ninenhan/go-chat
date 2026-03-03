package model

import (
	"gorm.io/datatypes"
)

// TaskIOType 描述输入/输出数据类型，可拓展为 CSV/JSON/DB 等。
type TaskIOType int8

// TaskStatus 描述任务执行状态。
type TaskStatus int8

const (
	TaskStatusPending   TaskStatus = 0 // 未开始
	TaskStatusSubmitted TaskStatus = 1 // 已提交
	TaskStatusRunning   TaskStatus = 2 // 运行中
	TaskStatusStored    TaskStatus = 3 // 已入库
	TaskStatusFailed    TaskStatus = 4
	TaskStatusRetrying  TaskStatus = 5  // 重试中
	TaskStatusAccepted  TaskStatus = 10 //其实没有这个状态，这个状态是一个中间态，目前是一层审批，所以用不到
	TaskStatusRejected  TaskStatus = 13
	TaskStatusEmpty     TaskStatus = 11
	TaskStatusError     TaskStatus = 14  // 执行出错
	TaskStatusDone      TaskStatus = 100 // 已完成
)

func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusDone || s < 0 || s == TaskStatusRejected
}

// TaskExecMode 描述任务执行方式，同步/异步。
type TaskExecMode string

const (
	TaskExecModeSync  TaskExecMode = "SYNC"
	TaskExecModeAsync TaskExecMode = "ASYNC"
)

func (m TaskExecMode) IsValid() bool {
	return m == TaskExecModeSync || m == TaskExecModeAsync
}

// GpiTask 定义通用任务结构，可通过 feature / input / output 组合描述自定义任务。
type GpiTask struct {
	GormModel
	Name         string         `gorm:"column:name;type:varchar(32);not null;default:''" json:"name"`
	Type         int8           `gorm:"column:type;type:tinyint;not null;default:0" json:"type"`
	ExecMode     TaskExecMode   `gorm:"column:exec_mode;type:varchar(8);not null;default:'ASYNC'" json:"execMode"`
	Feature      datatypes.JSON `gorm:"column:feature;type:json" json:"feature,omitempty"`
	InputSource  string         `gorm:"column:input_source;type:varchar(2048);not null" json:"inputSource"`
	InputType    TaskIOType     `gorm:"column:input_type;type:tinyint;not null;default:0" json:"inputType"`
	OutputType   TaskIOType     `gorm:"column:output_type;type:tinyint;not null;default:0" json:"outputType"`
	OutputSource string         `gorm:"column:output_source;type:varchar(2048)" json:"outputSource,omitempty"`
	Progress     int            `gorm:"column:progress;type:int;default:0" json:"progress"`
	Total        int            `gorm:"column:total;type:int;default:0" json:"total"`
	TenantId     string         `gorm:"column:tenant_id;type:varchar(64)" json:"tenantId,omitempty"`
	TaskStatus   TaskStatus     `gorm:"column:task_status;type:tinyint;not null;default:0" json:"taskStatus"`
	Status       int            `gorm:"column:status;type:tinyint;not null;default:0" json:"status"`
	GormAuditModel
}

func (GpiTask) TableName() string {
	return "gpi_task"
}
