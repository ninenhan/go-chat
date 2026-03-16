package service

import "github.com/ninenhan/go-chat/model"

func cloneAttachmentPtr(in *model.GenerationAttachment) *model.GenerationAttachment {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneHeaderMap(in map[string][]string) map[string][]string {
	if in == nil {
		return nil
	}
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
