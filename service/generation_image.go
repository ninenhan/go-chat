package service

import (
	"context"
	"errors"
	"strings"

	"github.com/ninenhan/go-chat/model"
)

func (s *DefaultGenerationService) GenerateImage(ctx context.Context, req *model.GenerationImageRequest) (*model.GenerationImageResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req == nil {
		return nil, errors.New("request 不能为空")
	}
	if strings.TrimSpace(req.TaskID) == "" {
		req.TaskID = s.cfg.TaskIDGenerator()
	}
	inputImages := normalizeImageInputImages(req)
	if !req.TaskType.IsValid() {
		if strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 {
			req.TaskType = model.GenerationTaskTypeImageGenerate
		} else if len(inputImages) > 0 || req.MaskImage != nil {
			req.TaskType = model.GenerationTaskTypeImageEdit
		} else {
			req.TaskType = model.GenerationTaskTypeImageGenerate
		}
	}
	if req.TaskType == model.GenerationTaskTypeTextChat {
		return nil, errors.New("GenerateImage 不支持 text_chat")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, errors.New("model 不能为空")
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		return nil, errors.New("prompt 不能为空")
	}
	req.InputImages = inputImages
	caller := s.cfg.ImageCaller
	if caller == nil {
		return nil, errors.New("image caller 未配置")
	}
	resp, err := caller(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("image caller 返回空响应")
	}
	if resp.TaskID == "" {
		resp.TaskID = req.TaskID
	}
	if !resp.TaskType.IsValid() {
		resp.TaskType = req.TaskType
	}
	if strings.TrimSpace(resp.Prompt) == "" {
		resp.Prompt = req.Prompt
	}
	return resp, nil
}
