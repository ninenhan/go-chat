package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ninenhan/go-chat/core"
	"github.com/ninenhan/go-chat/model"
)

func (s *DefaultGenerationService) StartSession(ctx context.Context, req *model.GenerationSessionStartRequest) (*model.GenerationSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req == nil {
		return nil, errors.New("request 不能为空")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = s.cfg.TaskIDGenerator()
	}
	ctxLimit := req.ContextLimitTokens
	if ctxLimit <= 0 {
		ctxLimit = s.cfg.ContextLimitTokens
	}
	now := time.Now()
	session := &model.GenerationSession{
		SessionID:          sessionID,
		TaskID:             sessionID,
		Model:              strings.TrimSpace(req.Model),
		Status:             model.GenerationSessionActive,
		SystemPrompts:      trimNonEmpty(req.SystemPrompts),
		ContextLimitTokens: ctxLimit,
		Messages:           []model.GenerationMessage{},
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.saveSession(ctx, session); err != nil {
		return nil, err
	}
	return cloneGenerationSession(session), nil
}

func (s *DefaultGenerationService) GetSession(ctx context.Context, sessionID string) (*model.GenerationSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("sessionID 不能为空")
	}
	return s.loadSession(ctx, sessionID)
}

func (s *DefaultGenerationService) ChatSession(ctx context.Context, req *model.GenerationSessionChatRequest, onDelta func(delta string) error) (*model.GenerationSessionChatResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req == nil {
		return nil, errors.New("request 不能为空")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return nil, errors.New("sessionID 不能为空")
	}
	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	prompt, err := s.BuildPrompt(&model.GenerationGenerateRequest{
		Prompt:       req.Prompt,
		Template:     req.Template,
		TemplateVars: req.TemplateVars,
	})
	if err != nil {
		return nil, err
	}

	modelCode := strings.TrimSpace(req.Model)
	if modelCode == "" {
		modelCode = strings.TrimSpace(session.Model)
	}
	if modelCode == "" {
		return nil, errors.New("model 不能为空")
	}
	session.Model = modelCode
	ctxLimit := req.ContextLimitTokens
	if ctxLimit <= 0 {
		ctxLimit = session.ContextLimitTokens
	}
	if ctxLimit <= 0 {
		ctxLimit = s.cfg.ContextLimitTokens
	}
	session.ContextLimitTokens = ctxLimit

	userMsg := model.GenerationMessage{
		MessageID:           fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Role:                model.GenerationRoleUser,
		Content:             prompt,
		ReferenceMessageIDs: append([]string(nil), req.ReferenceMessageIDs...),
		Tokens:              s.estimateTokens(prompt),
		CreatedAt:           time.Now(),
	}
	llmMessages, usedSummary, contextTokens, err := s.prepareSessionMessages(ctx, session, req.SystemPrompts, req.ReferenceMessageIDs, userMsg, ctxLimit)
	if err != nil {
		return nil, s.failAndMerge(ctx, session.TaskID, err)
	}
	if err := s.persistRunning(ctx, session.TaskID, prompt, modelCode); err != nil {
		return nil, err
	}

	stream := req.Stream || onDelta != nil
	xReq := NewChatXRequest(
		req.BaseURL,
		req.APIKey,
		modelCode,
		llmMessages,
		stream,
		req.ExtraHeaders,
		req.ExtraBody,
	)
	resultCh, err := s.callChat(ctx, xReq)
	if err != nil {
		return nil, s.failAndMerge(ctx, session.TaskID, err)
	}

	resp := &model.GenerationSessionChatResponse{
		SessionID:     session.SessionID,
		TaskID:        session.TaskID,
		UserMessageID: userMsg.MessageID,
		Prompt:        prompt,
		Status:        model.GenerationTaskRunning,
		UsedSummary:   usedSummary,
		Summary:       session.Summary,
		ContextTokens: contextTokens,
	}

	var builder strings.Builder
	chunks := make([]string, 0)
	hasOutput := false

	for item := range resultCh {
		switch v := item.(type) {
		case error:
			return nil, s.failAndMerge(ctx, session.TaskID, v)
		case core.ChatGPTResponse:
			resp.Raw = v
			resp.Output = v.GetResponse()
			hasOutput = strings.TrimSpace(resp.Output) != ""
		case core.ChatGPTStreamResponse:
			delta := v.GetResponse()
			if delta == "" {
				continue
			}
			builder.WriteString(delta)
			chunks = append(chunks, delta)
			hasOutput = true
			if onDelta != nil {
				if err := onDelta(delta); err != nil {
					return nil, s.failAndMerge(ctx, session.TaskID, err)
				}
			}
		default:
			err := fmt.Errorf("不支持的响应类型: %T", item)
			return nil, s.failAndMerge(ctx, session.TaskID, err)
		}
	}

	if stream {
		resp.Output = builder.String()
		if len(chunks) > 0 {
			resp.Chunks = chunks
		}
	}
	if !hasOutput {
		err := errors.New("模型未返回有效内容")
		return nil, s.failAndMerge(ctx, session.TaskID, err)
	}

	assistantMsg := model.GenerationMessage{
		MessageID: fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Role:      model.GenerationRoleAssistant,
		Content:   resp.Output,
		Tokens:    s.estimateTokens(resp.Output),
		CreatedAt: time.Now(),
	}
	if err := s.appendSessionMessage(ctx, session, userMsg); err != nil {
		return nil, err
	}
	if err := s.appendSessionMessage(ctx, session, assistantMsg); err != nil {
		return nil, err
	}
	if err := s.persistComplete(ctx, session.TaskID, resp.Output); err != nil {
		return nil, err
	}

	resp.AssistantMessageID = assistantMsg.MessageID
	resp.Status = model.GenerationTaskCompleted
	return resp, nil
}

func (s *DefaultGenerationService) prepareSessionMessages(
	ctx context.Context,
	session *model.GenerationSession,
	requestSystemPrompts []string,
	referenceMessageIDs []string,
	userMsg model.GenerationMessage,
	limit int,
) ([]map[string]string, bool, int, error) {
	available := limit - s.cfg.ReservedOutputTokens
	if available <= 0 {
		available = limit
	}
	if available <= 0 {
		available = s.cfg.ContextLimitTokens
	}

	references := collectReferenceMessages(session.Messages, referenceMessageIDs)
	usedSummary := false
	messages, usedTokens := s.composeSessionMessages(session, requestSystemPrompts, references, session.Messages, userMsg)
	if usedTokens <= available {
		return messages, strings.TrimSpace(session.Summary) != "", usedTokens, nil
	}

	// 先裁剪历史，只保留尾部最近消息。
	history := session.Messages
	selected := make([]model.GenerationMessage, 0, len(history))
	selectedTokens := s.estimateMessageTokens(userMsg.Role, userMsg.Content)
	baseTokens := usedTokens
	for _, h := range history {
		baseTokens -= s.estimateMessageTokens(h.Role, h.Content)
	}
	selectedTokens += baseTokens
	for i := len(history) - 1; i >= 0; i-- {
		h := history[i]
		ht := s.estimateMessageTokens(h.Role, h.Content)
		if selectedTokens+ht > available {
			break
		}
		selectedTokens += ht
		selected = append(selected, h)
	}
	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}

	droppedCount := len(history) - len(selected)
	if droppedCount > 0 {
		dropped := history[:droppedCount]
		summary, err := s.summarizeMessages(ctx, session, dropped)
		if err == nil && strings.TrimSpace(summary) != "" && summary != session.Summary {
			session.Summary = summary
			session.UpdatedAt = time.Now()
			if err := s.updateSessionSummary(ctx, session.SessionID, summary); err != nil {
				return nil, false, 0, err
			}
			usedSummary = true
		}
	}

	finalMessages, finalTokens := s.composeSessionMessages(session, requestSystemPrompts, references, selected, userMsg)
	if finalTokens > available {
		// 仍然超限时，直接仅保留 summary + 最近用户输入。
		finalMessages, finalTokens = s.composeSessionMessages(session, requestSystemPrompts, references, nil, userMsg)
	}
	if strings.TrimSpace(session.Summary) != "" {
		usedSummary = true
	}
	return finalMessages, usedSummary, finalTokens, nil
}

func (s *DefaultGenerationService) composeSessionMessages(
	session *model.GenerationSession,
	requestSystemPrompts []string,
	references []model.GenerationMessage,
	history []model.GenerationMessage,
	userMsg model.GenerationMessage,
) ([]map[string]string, int) {
	messages := make([]map[string]string, 0, len(history)+len(references)+8)
	totalTokens := 0

	sysPrompts := append([]string{}, session.SystemPrompts...)
	sysPrompts = append(sysPrompts, requestSystemPrompts...)
	sysPrompts = trimNonEmpty(sysPrompts)
	for _, sp := range sysPrompts {
		messages = append(messages, map[string]string{"role": string(model.GenerationRoleSystem), "content": sp})
		totalTokens += s.estimateMessageTokens(model.GenerationRoleSystem, sp)
	}

	if sum := strings.TrimSpace(session.Summary); sum != "" {
		summaryText := "历史会话总结:\n" + sum
		messages = append(messages, map[string]string{"role": string(model.GenerationRoleSystem), "content": summaryText})
		totalTokens += s.estimateMessageTokens(model.GenerationRoleSystem, summaryText)
	}

	if len(references) > 0 {
		lines := make([]string, 0, len(references))
		for _, ref := range references {
			content := strings.TrimSpace(ref.Content)
			if content == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("[%s][%s] %s", ref.MessageID, ref.Role, content))
		}
		if len(lines) > 0 {
			refText := "可引用的历史消息片段:\n" + strings.Join(lines, "\n")
			messages = append(messages, map[string]string{"role": string(model.GenerationRoleSystem), "content": refText})
			totalTokens += s.estimateMessageTokens(model.GenerationRoleSystem, refText)
		}
	}

	for _, h := range history {
		content := strings.TrimSpace(h.Content)
		if content == "" {
			continue
		}
		role := string(h.Role)
		if role == "" {
			role = string(model.GenerationRoleUser)
		}
		messages = append(messages, map[string]string{"role": role, "content": content})
		totalTokens += s.estimateMessageTokens(h.Role, content)
	}

	messages = append(messages, map[string]string{"role": string(model.GenerationRoleUser), "content": userMsg.Content})
	totalTokens += s.estimateMessageTokens(model.GenerationRoleUser, userMsg.Content)
	return messages, totalTokens
}

func (s *DefaultGenerationService) summarizeMessages(ctx context.Context, session *model.GenerationSession, messages []model.GenerationMessage) (string, error) {
	if len(messages) == 0 {
		return session.Summary, nil
	}
	if s.cfg.Summarizer != nil {
		result, err := s.cfg.Summarizer(ctx, GenerationSummarizeRequest{
			SessionID:       session.SessionID,
			ExistingSummary: session.Summary,
			Messages:        append([]model.GenerationMessage(nil), messages...),
			LimitTokens:     session.ContextLimitTokens,
		})
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(result), nil
	}
	// fallback: 本地压缩摘要，不调用外部模型。
	keep := messages
	if len(keep) > 12 {
		keep = keep[len(keep)-12:]
	}
	parts := make([]string, 0, len(keep)+1)
	if strings.TrimSpace(session.Summary) != "" {
		parts = append(parts, strings.TrimSpace(session.Summary))
	}
	for _, m := range keep {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		content = strings.ReplaceAll(content, "\n", " ")
		runes := []rune(content)
		if len(runes) > 180 {
			content = string(runes[:180]) + "..."
		}
		parts = append(parts, fmt.Sprintf("[%s] %s", m.Role, content))
	}
	result := strings.Join(parts, "\n")
	runes := []rune(result)
	if len(runes) > 3000 {
		result = string(runes[len(runes)-3000:])
	}
	return strings.TrimSpace(result), nil
}

func (s *DefaultGenerationService) estimateMessageTokens(role model.GenerationMessageRole, content string) int {
	base := 4
	if role == model.GenerationRoleSystem {
		base = 6
	}
	return base + s.estimateTokens(content)
}

func trimNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func collectReferenceMessages(history []model.GenerationMessage, ids []string) []model.GenerationMessage {
	if len(ids) == 0 || len(history) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			wanted[id] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	refs := make([]model.GenerationMessage, 0, len(wanted))
	for _, msg := range history {
		if _, ok := wanted[msg.MessageID]; ok {
			refs = append(refs, msg)
		}
	}
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].CreatedAt.Before(refs[j].CreatedAt) })
	return refs
}

func (s *DefaultGenerationService) sessionStore() GenerationSessionStore {
	if s.cfg.SessionStore != nil {
		return s.cfg.SessionStore
	}
	return nil
}

func (s *DefaultGenerationService) saveSession(ctx context.Context, session *model.GenerationSession) error {
	if session == nil || strings.TrimSpace(session.SessionID) == "" {
		return errors.New("session 不能为空")
	}
	if store := s.sessionStore(); store != nil {
		return store.SaveSession(ctx, cloneGenerationSession(session))
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.sessions[session.SessionID] = cloneGenerationSession(session)
	return nil
}

func (s *DefaultGenerationService) loadSession(ctx context.Context, sessionID string) (*model.GenerationSession, error) {
	if store := s.sessionStore(); store != nil {
		session, err := store.GetSession(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		if session == nil {
			return nil, errors.New("session 不存在")
		}
		return cloneGenerationSession(session), nil
	}
	s.sessionMu.RLock()
	defer s.sessionMu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, errors.New("session 不存在")
	}
	return cloneGenerationSession(session), nil
}

func (s *DefaultGenerationService) appendSessionMessage(ctx context.Context, session *model.GenerationSession, msg model.GenerationMessage) error {
	if session == nil {
		return errors.New("session 不能为空")
	}
	session.Messages = append(session.Messages, msg)
	session.UpdatedAt = time.Now()
	if store := s.sessionStore(); store != nil {
		if err := store.SaveSessionMessage(ctx, session.SessionID, &msg); err != nil {
			return err
		}
		return store.SaveSession(ctx, cloneGenerationSession(session))
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.sessions[session.SessionID] = cloneGenerationSession(session)
	return nil
}

func (s *DefaultGenerationService) updateSessionSummary(ctx context.Context, sessionID, summary string) error {
	if store := s.sessionStore(); store != nil {
		return store.UpdateSessionSummary(ctx, sessionID, summary)
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return errors.New("session 不存在")
	}
	session.Summary = summary
	session.UpdatedAt = time.Now()
	return nil
}

func cloneGenerationSession(session *model.GenerationSession) *model.GenerationSession {
	if session == nil {
		return nil
	}
	out := *session
	out.SystemPrompts = append([]string(nil), session.SystemPrompts...)
	if session.Messages != nil {
		out.Messages = make([]model.GenerationMessage, 0, len(session.Messages))
		for _, msg := range session.Messages {
			copyMsg := msg
			copyMsg.ReferenceMessageIDs = append([]string(nil), msg.ReferenceMessageIDs...)
			out.Messages = append(out.Messages, copyMsg)
		}
	}
	return &out
}
