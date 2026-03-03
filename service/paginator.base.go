package service

import (
	"feo.vip/chat/model"
	"gorm.io/gorm"
	"math"
)

// Paginate 对任意 T 类型做分页查询。
//   - db: 已经带了 Table／Order／Where etc 的 *gorm.DB
//   - m: 前端传来的分页和条件
//   - dest: 接收结果的指针，必须是 *[]T
//   - apply: 可选的额外条件函数列表
func Paginate[T any](
	db *gorm.DB,
	m *model.PageRO,
	dest *[]T,
	apply ...func(*gorm.DB) *gorm.DB,
) (*model.PageVO[T], error) {
	// 1. 默认分页参数
	if m.PageSize <= 0 {
		m.PageSize = 10
	}
	if m.PageNum <= 0 {
		m.PageNum = 1
	}

	// 2. 应用额外条件
	for _, fn := range apply {
		db = fn(db)
	}

	// 3. 先 Count
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	// 4. 如果没数据，返回空分页对象
	if total == 0 {
		return &model.PageVO[T]{
			List:       []T{},
			Total:      0,
			PageSize:   m.PageSize,
			PageNum:    m.PageNum,
			TotalPages: 0,
		}, nil
	}

	// 5. 分页查询
	offset := (m.PageNum - 1) * m.PageSize
	if err := db.Offset(offset).Limit(m.PageSize).Find(dest).Error; err != nil {
		return nil, err
	}

	// 6. 构建返回
	totalPages := int(math.Ceil(float64(total) / float64(m.PageSize)))
	return &model.PageVO[T]{
		List:       *dest,
		Total:      total,
		PageSize:   m.PageSize,
		PageNum:    m.PageNum,
		TotalPages: totalPages,
	}, nil
}
