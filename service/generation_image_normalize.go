package service

import (
	"strings"

	"github.com/ninenhan/go-chat/model"
)

func normalizeSessionInputImages(req *model.GenerationSessionChatRequest) []model.GenerationAttachment {
	if req == nil {
		return nil
	}
	if len(req.Attachments) > 0 {
		return append([]model.GenerationAttachment(nil), req.Attachments...)
	}
	items := make([]model.GenerationAttachment, 0, 1+len(req.Images))
	if uri := strings.TrimSpace(req.Image); uri != "" {
		items = append(items, model.GenerationAttachment{Kind: "image", URI: uri})
	}
	for _, uri := range req.Images {
		uri = strings.TrimSpace(uri)
		if uri == "" {
			continue
		}
		items = append(items, model.GenerationAttachment{Kind: "image", URI: uri})
	}
	return items
}

func normalizeImageInputImages(req *model.GenerationImageRequest) []model.GenerationAttachment {
	if req == nil {
		return nil
	}
	if len(req.InputImages) > 0 {
		return append([]model.GenerationAttachment(nil), req.InputImages...)
	}
	items := make([]model.GenerationAttachment, 0, 1+len(req.Images))
	if uri := strings.TrimSpace(req.Image); uri != "" {
		items = append(items, model.GenerationAttachment{Kind: "image", URI: uri})
	}
	for _, uri := range req.Images {
		uri = strings.TrimSpace(uri)
		if uri == "" {
			continue
		}
		items = append(items, model.GenerationAttachment{Kind: "image", URI: uri})
	}
	return items
}

func resolveSessionImageSize(req *model.GenerationSessionChatRequest) string {
	if req == nil {
		return ""
	}
	if v := strings.TrimSpace(req.ImageSize); v != "" {
		return v
	}
	return strings.TrimSpace(req.Size)
}

func resolveSessionImageCount(req *model.GenerationSessionChatRequest) int {
	if req == nil {
		return 0
	}
	if req.ImageCount > 0 {
		return req.ImageCount
	}
	return req.N
}

func mergeImageExtraBody(base map[string]any, outputFormat string, watermark *bool) map[string]any {
	out := cloneAnyMap(base)
	if out == nil {
		out = map[string]any{}
	}
	if v := strings.TrimSpace(outputFormat); v != "" {
		if _, ok := out["output_format"]; !ok {
			out["output_format"] = v
		}
	}
	if watermark != nil {
		if _, ok := out["watermark"]; !ok {
			out["watermark"] = *watermark
		}
	}
	return out
}
