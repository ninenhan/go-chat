package model

import (
	"github.com/ninenhan/go-workflow/flow"
)

type SorterRO struct {
	Column string `json:"column"`
	Desc   *bool  `json:"desc"` // 默认为 false
}

type PageRO struct {
	PageNum         int              `json:"pageNum"`
	PageSize        int              `json:"pageSize"`
	Keyword         string           `json:"keyword"`
	Condition       []flow.Condition `json:"condition"`
	FilterCondition []flow.Condition `json:"filterCondition"`
	Sorts           []SorterRO       `json:"sorts"`
	RequestId       string           `json:"requestId"`
}

type MapExtends = map[string]any
type PageVO[T any] struct {
	PageNum    int         `json:"pageNum,omitempty"`
	PageSize   int         `json:"pageSize,omitempty"`
	Total      int64       `json:"total,omitempty"`
	List       []T         `json:"list,omitempty"`
	PageRO     *PageRO     `json:"page,omitempty"`
	TotalPages int         `json:"totalPages,omitempty"`
	RequestId  string      `json:"requestId,omitempty"`
	Extends    *MapExtends `json:"extends,omitempty"`
}

func PageVoMap[T any, R any](p *PageVO[T], mapper func(T) R) *PageVO[R] {
	// 预分配目标切片
	mapped := make([]R, 0, len(p.List))
	for _, item := range p.List {
		mapped = append(mapped, mapper(item))
	}
	return &PageVO[R]{
		List:       mapped,
		Total:      p.Total,
		PageNum:    p.PageNum,
		PageSize:   p.PageSize,
		TotalPages: p.TotalPages,
	}
}

func (p *PageRO) ZeroPageFixed() {
	p.PageNum = p.PageNum - 1
	if p.PageNum < 0 {
		p.PageNum = 0
	}
}
func (p *PageRO) NormalPageFixed() {
	if p.PageNum <= 0 {
		p.PageNum = 1
	}
}
