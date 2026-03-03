package service

import (
	"context"
	"testing"
	"time"

	"github.com/ninenhan/go-chat/model"
)

func TestGenerationTemplateSlots(t *testing.T) {
	svc := NewGenerationService()
	tpl := "我需要{{ 品牌 }}平台文案，卖点：{{卖点}}，需求：{{需求}}"
	slots, err := svc.ParseTemplateSlots(tpl)
	if err != nil {
		t.Fatalf("ParseTemplateSlots error: %v", err)
	}
	if len(slots) != 3 {
		t.Fatalf("unexpected slot count: %d", len(slots))
	}
}

func TestGenerationRenderTemplate(t *testing.T) {
	svc := NewGenerationService()
	tpl := "品牌{{品牌}}，卖点{{ 卖点 }}"
	out, err := svc.RenderTemplate(tpl, map[string]string{
		"品牌": "A",
		"卖点": "快",
	})
	if err != nil {
		t.Fatalf("RenderTemplate error: %v", err)
	}
	if out != "品牌A，卖点快" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestGenerationRenderTemplateMissingVars(t *testing.T) {
	svc := NewGenerationService()
	_, err := svc.RenderTemplate("品牌{{品牌}}，卖点{{卖点}}", map[string]string{"品牌": "A"})
	if err == nil {
		t.Fatal("expected missing var error")
	}
}

func TestGenerationRenderTemplateWithControl(t *testing.T) {
	svc := NewGenerationService()
	tpl := "<% if number(score) >= 60 %>通过<% else %>未通过<% end %>，{{name}}"
	out, err := svc.RenderTemplate(tpl, map[string]string{
		"name":  "张三",
		"score": "88",
	})
	if err != nil {
		t.Fatalf("RenderTemplateWithControl error: %v", err)
	}
	if out != "通过，张三" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestInMemoryGenerationStoreSession(t *testing.T) {
	store := NewInMemoryGenerationStore()
	ctx := context.Background()
	now := time.Now()
	session := &model.GenerationSession{
		SessionID: "s1",
		TaskID:    "t1",
		Model:     "gpt-test",
		Status:    model.GenerationSessionActive,
		Messages: []model.GenerationMessage{
			{MessageID: "m1", Role: model.GenerationRoleUser, Content: "hello", CreatedAt: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession error: %v", err)
	}
	got, err := store.GetSession(ctx, "s1")
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	if got.SessionID != "s1" || len(got.Messages) != 1 || got.Messages[0].Content != "hello" {
		t.Fatalf("unexpected session payload: %+v", got)
	}
}
