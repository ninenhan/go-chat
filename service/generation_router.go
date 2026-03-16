package service

import (
	"context"
	"strings"

	"github.com/ninenhan/go-chat/model"
)

func defaultGenerationTaskResolver(_ context.Context, session *model.GenerationSession, req *model.GenerationSessionChatRequest) model.GenerationTaskType {
	if req != nil && req.TurnType.IsValid() {
		return req.TurnType
	}
	if req != nil {
		if strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 {
			return model.GenerationTaskTypeImageGenerate
		}
		if len(req.Attachments) > 0 || req.MaskImage != nil {
			return model.GenerationTaskTypeImageEdit
		}
	}
	if session != nil && session.DefaultTaskType.IsValid() {
		return session.DefaultTaskType
	}
	return model.GenerationTaskTypeTextChat
}

func (s *DefaultGenerationService) resolveSessionTaskType(
	ctx context.Context,
	session *model.GenerationSession,
	req *model.GenerationSessionChatRequest,
) model.GenerationTaskType {
	resolver := s.cfg.TaskResolver
	if resolver == nil {
		resolver = defaultGenerationTaskResolver
	}
	taskType := resolver(ctx, session, req)
	if !taskType.IsValid() {
		taskType = model.GenerationTaskTypeTextChat
	}
	return taskType
}

func defaultImageOutputText(artifacts []model.GenerationArtifact) string {
	if len(artifacts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(artifacts))
	for _, item := range artifacts {
		uri := strings.TrimSpace(item.URI)
		if uri == "" {
			continue
		}
		parts = append(parts, uri)
	}
	return strings.Join(parts, "\n")
}
