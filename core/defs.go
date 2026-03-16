package core

import (
	"errors"
	"github.com/ninenhan/go-workflow/flow"
	"github.com/ninenhan/go-workflow/fn"
	"log/slog"
	"strconv"
	"strings"
)

//region 【1】 Endpoint 端点
/**
 * 端点描述以何种方式、何种内容去访问指定的资源
 */

type EndpointType = string

var (
	EndpointType_HTTP EndpointType = "HTTP"
)

type IEndpoint[T any] interface {
	GetType() EndpointType
	GetEndPoint() T
	IsValid() bool
}

type Endpoint struct {
	Id          string       `json:"id,omitempty" bson:"id,omitempty"`
	Name        string       `json:"name,omitempty" bson:"name,omitempty"`
	Description string       `json:"description,omitempty" bson:"description,omitempty"`
	Type        EndpointType `json:"type,omitempty" bson:"type,omitempty"`
	Payload     any          `json:"payload,omitempty" bson:"payload,omitempty"`
}

type IPayloadValidator interface {
	IsValid() bool
}

type HttpEndpointPayload struct {
	Url     string              `json:"url,omitempty" bson:"url,omitempty"`
	Method  string              `json:"method,omitempty" bson:"method,omitempty"`
	Headers map[string][]string `json:"headers,omitempty" bson:"headers,omitempty"`
	Body    any                 `json:"body,omitempty" bson:"body,omitempty"`
	//Query   map[string]string   `json:"query,omitempty" bson:"query,omitempty"`
	Stream bool `json:"stream,omitempty" bson:"stream,omitempty"`
}

func (p *HttpEndpointPayload) IsValid() bool {
	return p.Url != ""
}

type HttpEndpoint struct {
	Endpoint `json:",omitempty,inline" bson:",inline"`
	Payload  *HttpEndpointPayload `json:"payload,omitempty" bson:"payload,omitempty"`
}

func (e *HttpEndpoint) GetType() EndpointType {
	return e.Type
}
func (e *HttpEndpoint) GetEndPoint() HttpEndpoint {
	return *e
}

func (e *HttpEndpoint) IsValid() bool {
	if strings.Contains(e.Name, "->") || strings.Contains(e.Name, "|") {
		return false
	}
	return e.Payload.IsValid()
}

type EndpointList = []IEndpoint[any]
type HttpEndpointList = []IEndpoint[HttpEndpoint]

//endregion

type PipeType = string

var (
	PipeTypeDefault              PipeType = "DEFAULT"
	PipeTypeNonStreamPlaintext   PipeType = "NON_STREAM_PLAINTEXT"
	PipeTypeTextBreakCollectList PipeType = "TEXT_BREAK_COLLECT_LIST"
	PipeTypeListDistinct         PipeType = "LIST_DISTINCT"
	PipeTypeStringTrim           PipeType = "STR_TRIM"
	PipeTypeStringToNumber       PipeType = "STR_TO_INT"
	PipeTypeStringToDecimal      PipeType = "STR_TO_DECIMAL"
)

type Pipe struct {
	Type *PipeType `json:"type,omitempty"`
}

func (p *Pipe) DoPipe(input any) (any, error) {
	if p.Type == nil || *p.Type == PipeTypeDefault {
		return input, nil
	}
	slog.Info("PipeType: %s", "type", p.Type)
	switch *p.Type {
	case PipeTypeNonStreamPlaintext:
		if input, ok := input.(ChatGPTResponse); ok {
			return input.GetResponse(), nil
		}
	case PipeTypeTextBreakCollectList:
		if input, ok := input.(string); ok {
			return strings.Split(input, "\n"), nil
		}
	case PipeTypeListDistinct:
		if input, ok := input.([]string); ok {
			distinctList := fn.UniqueList(input)
			return distinctList, nil
		}
	case PipeTypeStringTrim:
		if input, ok := input.(string); ok {
			return strings.TrimSpace(input), nil
		}
	case PipeTypeStringToNumber:
		if input, ok := input.(string); ok {
			//转INT
			num, err := strconv.Atoi(input)
			if err == nil {
				// 处理错误
				return num, nil
			}
		}
	case PipeTypeStringToDecimal:
		if input, ok := input.(string); ok {
			num, err := strconv.ParseFloat(input, 64) // 64 表示解析为 float64
			if err == nil {
				// 处理错误
				return num, nil
			}
		}
	}
	return nil, errors.New("无效的输入类型, AT : " + (*p.Type))
}

//region 【2】 EndpointWorker

type EndpointWorker struct {
	Endpoint EndpointList `json:"endpoint,omitempty"`
	WorkerId string       `json:"worker_id,omitempty"`
}

type EndpointSelector struct {
	XRequest *XRequest        `json:"request,omitempty"`
	Pipes    []PipeType       `json:"pipes,omitempty"`
	Selector []flow.Condition `json:"selector,omitempty"`
}

//endregion
