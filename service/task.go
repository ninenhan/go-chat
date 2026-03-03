package service

import (
	"errors"
	"feo.vip/chat/model"
	"gorm.io/gorm"
	"log"
)

// TaskService 提供通用任务的增改与进度维护能力。
type TaskService struct {
	//必须私有，防止名称被占用
	orm   *gorm.DB
	table string
}

// NewTaskService 自动迁移任务表，并返回服务实例。
func NewTaskService(db *gorm.DB, table ...string) *TaskService {
	targetTable := ""
	if len(table) > 0 {
		targetTable = table[0]
	}
	return NewTaskServiceWithTable(db, targetTable)
}

// NewTaskServiceWithTable 自动迁移任务表，并返回服务实例。
func NewTaskServiceWithTable(db *gorm.DB, table string) *TaskService {
	if table == "" {
		table = model.GpiTask{}.TableName() // 默认表
	}
	if err := db.Table(table).AutoMigrate(&model.GpiTask{}); err != nil {
		log.Fatalf("初始化任务表失败: %v", err)
	}
	return &TaskService{orm: db, table: table}
}

func (s *TaskService) q() *gorm.DB {
	return s.orm.Table(s.table)
}

// CreateTask 保存通用任务。
func (s *TaskService) CreateTask(task *model.GpiTask) (*model.GpiTask, error) {
	if task == nil {
		return nil, errors.New("task 不能为空")
	}
	if err := validateTask(task); err != nil {
		return nil, err
	}
	if err := s.q().Create(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

// UpdateTask 更新任务配置以及状态。
func (s *TaskService) UpdateTask(task *model.GpiTask) (*model.GpiTask, error) {
	if task == nil || task.ID == 0 {
		return nil, errors.New("task ID 不能为空")
	}
	var existing model.GpiTask
	if err := s.q().First(&existing, task.ID).Error; err != nil {
		return nil, err
	}
	if task.ExecMode == "" {
		task.ExecMode = existing.ExecMode
	}
	if existing.ExecMode != "" && task.ExecMode != existing.ExecMode {
		return nil, errors.New("任务执行模式创建后不可修改")
	}
	if err := validateTask(task); err != nil {
		return nil, err
	}
	task.Progress, task.Total = normalizeProgress(task.Progress, task.Total)
	updatePayload := map[string]any{
		"name":          task.Name,
		"type":          task.Type,
		"exec_mode":     task.ExecMode,
		"feature":       task.Feature,
		"input_source":  task.InputSource,
		"input_type":    task.InputType,
		"output_type":   task.OutputType,
		"output_source": task.OutputSource,
		"progress":      task.Progress,
		"total":         task.Total,
		"task_status":   task.TaskStatus,
		"status":        task.Status,
		"tenant_id":     task.TenantId,
		"updated_by":    task.UpdatedBy,
	}
	if err := s.q().Model(&model.GpiTask{}).
		Where("id = ?", task.ID).
		Updates(updatePayload).Error; err != nil {
		return nil, err
	}
	var updated model.GpiTask
	if err := s.q().First(&updated, task.ID).Error; err != nil {
		return nil, err
	}
	return &updated, nil
}

// UpdateProgress 快速刷新任务进度与状态，适合 Worker 调用。
func (s *TaskService) UpdateProgress(taskID uint64, progress, total int, status model.TaskStatus, updateUser string) error {
	if taskID == 0 {
		return errors.New("taskID 不能为空")
	}
	progress, total = normalizeProgress(progress, total)
	payload := map[string]any{
		"progress":    progress,
		"total":       total,
		"task_status": status,
	}
	if updateUser != "" {
		payload["update_user"] = updateUser
	}
	return s.q().Model(&model.GpiTask{}).
		Where("id = ?", taskID).
		Updates(payload).Error
}

func validateTask(task *model.GpiTask) error {
	if task.Name == "" {
		return errors.New("任务名称不能为空")
	}
	if task.InputSource == "" {
		return errors.New("输入数据来源不能为空")
	}
	if task.ExecMode == "" {
		task.ExecMode = model.TaskExecModeAsync
	}
	if !task.ExecMode.IsValid() {
		return errors.New("任务执行模式非法")
	}
	if task.CreatedBy == "" {
		task.CreatedBy = "-1"
	}
	if task.UpdatedBy == "" {
		task.UpdatedBy = task.CreatedBy
	}
	return nil
}

func normalizeProgress(progress, total int) (int, int) {
	if progress < 0 {
		progress = 0
	}
	if total < progress {
		total = progress
	}
	return progress, total
}
