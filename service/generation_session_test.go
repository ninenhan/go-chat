package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/ninenhan/go-chat/core"
	"github.com/ninenhan/go-chat/model"
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

func TestGenerationSessionRoutesImageTurnToImageCaller(t *testing.T) {
	store := NewInMemoryGenerationStore()
	chatCalled := false
	imageCalled := false

	chatCaller := func(_ context.Context, _ *core.XRequest) (chan any, error) {
		chatCalled = true
		ch := make(chan any, 1)
		close(ch)
		return ch, nil
	}
	imageCaller := func(_ context.Context, req *model.GenerationImageRequest) (*model.GenerationImageResponse, error) {
		imageCalled = true
		if req.TaskType != model.GenerationTaskTypeImageGenerate {
			t.Fatalf("unexpected taskType: %s", req.TaskType)
		}
		return &model.GenerationImageResponse{
			TaskID:   req.TaskID,
			TaskType: req.TaskType,
			Status:   model.GenerationTaskCompleted,
			Prompt:   req.Prompt,
			Artifacts: []model.GenerationArtifact{
				{Kind: "image", URI: "https://img.local/1.png"},
			},
		}, nil
	}

	svc := NewGenerationService(
		WithGenerationSessionStore(store),
		WithGenerationQueue(false, 0, 0),
		WithGenerationChatCaller(chatCaller),
		WithGenerationImageCaller(imageCaller),
	)

	session, err := svc.StartSession(context.Background(), &model.GenerationSessionStartRequest{
		Model: "flux-test",
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}

	resp, err := svc.ChatSession(context.Background(), &model.GenerationSessionChatRequest{
		SessionID: session.SessionID,
		TurnType:  model.GenerationTaskTypeImageGenerate,
		Prompt:    "画一只戴墨镜的柴犬",
		BaseURL:   "https://mock.local",
		APIKey:    "x",
	}, nil)
	if err != nil {
		t.Fatalf("ChatSession error: %v", err)
	}
	if !imageCalled {
		t.Fatal("expected image caller to be called")
	}
	if chatCalled {
		t.Fatal("chat caller should not be called for image turn")
	}
	if resp.TaskType != model.GenerationTaskTypeImageGenerate {
		t.Fatalf("unexpected response taskType: %s", resp.TaskType)
	}
	if len(resp.Artifacts) != 1 || resp.Artifacts[0].URI != "https://img.local/1.png" {
		t.Fatalf("unexpected artifacts: %+v", resp.Artifacts)
	}
}

func TestGenerationSessionInfersImageEditFromAttachments(t *testing.T) {
	store := NewInMemoryGenerationStore()

	imageCaller := func(_ context.Context, req *model.GenerationImageRequest) (*model.GenerationImageResponse, error) {
		if req.TaskType != model.GenerationTaskTypeImageEdit {
			t.Fatalf("unexpected taskType: %s", req.TaskType)
		}
		if len(req.InputImages) != 1 || req.InputImages[0].URI != "https://img.local/input.png" {
			t.Fatalf("unexpected input images: %+v", req.InputImages)
		}
		return &model.GenerationImageResponse{
			TaskID:   req.TaskID,
			TaskType: req.TaskType,
			Status:   model.GenerationTaskCompleted,
			Prompt:   req.Prompt,
			Artifacts: []model.GenerationArtifact{
				{Kind: "image", URI: "https://img.local/output.png"},
			},
		}, nil
	}

	svc := NewGenerationService(
		WithGenerationSessionStore(store),
		WithGenerationQueue(false, 0, 0),
		WithGenerationImageCaller(imageCaller),
	)

	session, err := svc.StartSession(context.Background(), &model.GenerationSessionStartRequest{
		Model: "flux-test",
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}

	resp, err := svc.ChatSession(context.Background(), &model.GenerationSessionChatRequest{
		SessionID: session.SessionID,
		Prompt:    "保留构图，改成赛博朋克风格",
		Attachments: []model.GenerationAttachment{
			{Kind: "image", URI: "https://img.local/input.png"},
		},
		BaseURL: "https://mock.local",
		APIKey:  "x",
	}, nil)
	if err != nil {
		t.Fatalf("ChatSession error: %v", err)
	}
	if resp.TaskType != model.GenerationTaskTypeImageEdit {
		t.Fatalf("unexpected response taskType: %s", resp.TaskType)
	}
}

func TestGenerationSessionInfersImageGenerateFromImageField(t *testing.T) {
	store := NewInMemoryGenerationStore()

	imageCaller := func(_ context.Context, req *model.GenerationImageRequest) (*model.GenerationImageResponse, error) {
		if req.TaskType != model.GenerationTaskTypeImageGenerate {
			t.Fatalf("unexpected taskType: %s", req.TaskType)
		}
		if req.Image != "https://img.local/input.png" {
			t.Fatalf("unexpected image field: %s", req.Image)
		}
		if req.Size != "2K" {
			t.Fatalf("unexpected size: %s", req.Size)
		}
		if got, ok := req.ExtraBody["output_format"].(string); !ok || got != "png" {
			t.Fatalf("unexpected output_format: %+v", req.ExtraBody["output_format"])
		}
		if got, ok := req.ExtraBody["watermark"].(bool); !ok || got {
			t.Fatalf("unexpected watermark: %+v", req.ExtraBody["watermark"])
		}
		return &model.GenerationImageResponse{
			TaskID:   req.TaskID,
			TaskType: req.TaskType,
			Status:   model.GenerationTaskCompleted,
			Prompt:   req.Prompt,
			Artifacts: []model.GenerationArtifact{
				{Kind: "image", URI: "https://img.local/output.png"},
			},
		}, nil
	}

	svc := NewGenerationService(
		WithGenerationSessionStore(store),
		WithGenerationQueue(false, 0, 0),
		WithGenerationImageCaller(imageCaller),
	)

	session, err := svc.StartSession(context.Background(), &model.GenerationSessionStartRequest{
		Model: "seedream",
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}
	watermark := false
	resp, err := svc.ChatSession(context.Background(), &model.GenerationSessionChatRequest{
		SessionID:    session.SessionID,
		Prompt:       "保持姿势不变，改成透明玻璃材质",
		Image:        "https://img.local/input.png",
		Size:         "2K",
		OutputFormat: "png",
		Watermark:    &watermark,
		BaseURL:      "https://mock.local",
		APIKey:       "x",
	}, nil)
	if err != nil {
		t.Fatalf("ChatSession error: %v", err)
	}
	if resp.TaskType != model.GenerationTaskTypeImageGenerate {
		t.Fatalf("unexpected response taskType: %s", resp.TaskType)
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
