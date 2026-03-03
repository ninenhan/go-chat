package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"feo.vip/chat/core"
	"feo.vip/chat/model"
)

func TestGenerationSessionMultiTurnWithReference(t *testing.T) {
	store := NewInMemoryGenerationStore()

	var mu sync.Mutex
	var lastReqMessages []map[string]string
	caller := func(_ context.Context, xReq *core.XRequest) (chan any, error) {
		mu.Lock()
		lastReqMessages = decodeMessages(xReq.Body)
		mu.Unlock()

		lastUser := ""
		for i := len(lastReqMessages) - 1; i >= 0; i-- {
			if lastReqMessages[i]["role"] == "user" {
				lastUser = lastReqMessages[i]["content"]
				break
			}
		}
		payload := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "reply:" + lastUser,
					},
				},
			},
		}
		var resp core.ChatGPTResponse
		bs, _ := json.Marshal(payload)
		_ = json.Unmarshal(bs, &resp)

		ch := make(chan any, 1)
		ch <- resp
		close(ch)
		return ch, nil
	}

	svc := NewGenerationService(
		WithGenerationSessionStore(store),
		WithGenerationQueue(false, 0, 0),
		WithGenerationChatCaller(caller),
	)

	session, err := svc.StartSession(context.Background(), &model.GenerationSessionStartRequest{
		Model:         "gpt-test",
		SystemPrompts: []string{"你是文案助手"},
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}

	r1, err := svc.ChatSession(context.Background(), &model.GenerationSessionChatRequest{
		SessionID: session.SessionID,
		Prompt:    "给我第一版标题",
		BaseURL:   "https://mock.local",
		APIKey:    "x",
	}, nil)
	if err != nil {
		t.Fatalf("ChatSession round1 error: %v", err)
	}
	if r1.AssistantMessageID == "" {
		t.Fatal("AssistantMessageID should not be empty")
	}

	_, err = svc.ChatSession(context.Background(), &model.GenerationSessionChatRequest{
		SessionID:           session.SessionID,
		Prompt:              "基于刚才的结果改短一点",
		ReferenceMessageIDs: []string{r1.AssistantMessageID},
		BaseURL:             "https://mock.local",
		APIKey:              "x",
	}, nil)
	if err != nil {
		t.Fatalf("ChatSession round2 error: %v", err)
	}

	gotSession, err := svc.GetSession(context.Background(), session.SessionID)
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	if len(gotSession.Messages) != 4 {
		t.Fatalf("unexpected message count: %d", len(gotSession.Messages))
	}

	mu.Lock()
	defer mu.Unlock()
	joined := joinMessageContent(lastReqMessages)
	if !strings.Contains(joined, r1.AssistantMessageID) {
		t.Fatalf("expected referenced message id in llm input, got: %s", joined)
	}
}

func TestGenerationSessionAutoSummary(t *testing.T) {
	store := NewInMemoryGenerationStore()
	summaryCalled := 0

	caller := func(_ context.Context, _ *core.XRequest) (chan any, error) {
		payload := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "ok",
					},
				},
			},
		}
		var resp core.ChatGPTResponse
		bs, _ := json.Marshal(payload)
		_ = json.Unmarshal(bs, &resp)
		ch := make(chan any, 1)
		ch <- resp
		close(ch)
		return ch, nil
	}

	svc := NewGenerationService(
		WithGenerationSessionStore(store),
		WithGenerationQueue(false, 0, 0),
		WithGenerationContext(64, 16),
		WithGenerationTokenEstimator(func(text string) int {
			return len([]rune(strings.TrimSpace(text)))
		}),
		WithGenerationSummarizer(func(_ context.Context, req GenerationSummarizeRequest) (string, error) {
			summaryCalled++
			return req.ExistingSummary + " | SUM", nil
		}),
		WithGenerationChatCaller(caller),
	)

	session, err := svc.StartSession(context.Background(), &model.GenerationSessionStartRequest{
		Model:              "gpt-test",
		ContextLimitTokens: 64,
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}

	for i := 0; i < 3; i++ {
		resp, err := svc.ChatSession(context.Background(), &model.GenerationSessionChatRequest{
			SessionID: session.SessionID,
			Prompt:    strings.Repeat("长文本", 20),
			BaseURL:   "https://mock.local",
			APIKey:    "x",
		}, nil)
		if err != nil {
			t.Fatalf("ChatSession error: %v", err)
		}
		if i == 2 && !resp.UsedSummary {
			t.Fatal("expected UsedSummary=true on late rounds")
		}
	}

	if summaryCalled == 0 {
		t.Fatal("expected summarizer to be called")
	}
	gotSession, err := svc.GetSession(context.Background(), session.SessionID)
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	if strings.TrimSpace(gotSession.Summary) == "" {
		t.Fatal("expected non-empty summary")
	}
}

func decodeMessages(body any) []map[string]string {
	obj, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := obj["messages"].([]map[string]string)
	if ok {
		return raw
	}

	arr, ok := obj["messages"].([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]string, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		result = append(result, map[string]string{"role": role, "content": content})
	}
	return result
}

func joinMessageContent(messages []map[string]string) string {
	parts := make([]string, 0, len(messages))
	for _, m := range messages {
		parts = append(parts, m["content"])
	}
	return strings.Join(parts, "\n")
}
