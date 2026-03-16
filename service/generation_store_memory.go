package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ninenhan/go-chat/model"
)

// InMemoryGenerationStore 是 GenerationSessionStore 的内存实现，适合本地开发或测试。
type InMemoryGenerationStore struct {
	mu       sync.RWMutex
	sessions map[string]*model.GenerationSession
}

func NewInMemoryGenerationStore() *InMemoryGenerationStore {
	return &InMemoryGenerationStore{
		sessions: make(map[string]*model.GenerationSession),
	}
}

func (s *InMemoryGenerationStore) SaveSession(_ context.Context, session *model.GenerationSession) error {
	if session == nil || session.SessionID == "" {
		return errors.New("session 不能为空且必须包含 sessionID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	existing, ok := s.sessions[session.SessionID]
	if !ok {
		cp := cloneSessionForStore(session)
		if cp.CreatedAt.IsZero() {
			cp.CreatedAt = now
		}
		if cp.UpdatedAt.IsZero() {
			cp.UpdatedAt = now
		}
		s.sessions[session.SessionID] = cp
		return nil
	}

	if session.TaskID != "" {
		existing.TaskID = session.TaskID
	}
	if session.Model != "" {
		existing.Model = session.Model
	}
	if session.DefaultTaskType.IsValid() {
		existing.DefaultTaskType = session.DefaultTaskType
	}
	if session.Status != "" {
		existing.Status = session.Status
	}
	if session.ContextLimitTokens > 0 {
		existing.ContextLimitTokens = session.ContextLimitTokens
	}
	if len(session.SystemPrompts) > 0 {
		existing.SystemPrompts = append([]string(nil), session.SystemPrompts...)
	}
	if session.Summary != "" {
		existing.Summary = session.Summary
	}
	if len(session.Messages) > 0 {
		existing.Messages = cloneMessagesForStore(session.Messages)
	}
	if !session.CreatedAt.IsZero() {
		existing.CreatedAt = session.CreatedAt
	}
	existing.UpdatedAt = now
	return nil
}

func (s *InMemoryGenerationStore) GetSession(_ context.Context, sessionID string) (*model.GenerationSession, error) {
	if sessionID == "" {
		return nil, errors.New("sessionID 不能为空")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, errors.New("session 不存在")
	}
	return cloneSessionForStore(session), nil
}

func (s *InMemoryGenerationStore) SaveSessionMessage(_ context.Context, sessionID string, message *model.GenerationMessage) error {
	if sessionID == "" {
		return errors.New("sessionID 不能为空")
	}
	if message == nil || message.MessageID == "" {
		return errors.New("message 不能为空且必须包含 messageID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		now := time.Now()
		session = &model.GenerationSession{
			SessionID: sessionID,
			TaskID:    sessionID,
			Status:    model.GenerationSessionActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		s.sessions[sessionID] = session
	}
	cp := *message
	cp.ReferenceMessageIDs = append([]string(nil), message.ReferenceMessageIDs...)
	cp.Attachments = append([]model.GenerationAttachment(nil), message.Attachments...)
	cp.Artifacts = append([]model.GenerationArtifact(nil), message.Artifacts...)
	session.Messages = append(session.Messages, cp)
	session.UpdatedAt = time.Now()
	return nil
}

func (s *InMemoryGenerationStore) UpdateSessionSummary(_ context.Context, sessionID, summary string) error {
	if sessionID == "" {
		return errors.New("sessionID 不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		now := time.Now()
		session = &model.GenerationSession{
			SessionID: sessionID,
			TaskID:    sessionID,
			Status:    model.GenerationSessionActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		s.sessions[sessionID] = session
	}
	session.Summary = summary
	session.UpdatedAt = time.Now()
	return nil
}

func cloneSessionForStore(in *model.GenerationSession) *model.GenerationSession {
	if in == nil {
		return nil
	}
	out := *in
	out.SystemPrompts = append([]string(nil), in.SystemPrompts...)
	out.Messages = cloneMessagesForStore(in.Messages)
	return &out
}

func cloneMessagesForStore(in []model.GenerationMessage) []model.GenerationMessage {
	if in == nil {
		return nil
	}
	out := make([]model.GenerationMessage, 0, len(in))
	for _, msg := range in {
		cp := msg
		cp.ReferenceMessageIDs = append([]string(nil), msg.ReferenceMessageIDs...)
		cp.Attachments = append([]model.GenerationAttachment(nil), msg.Attachments...)
		cp.Artifacts = append([]model.GenerationArtifact(nil), msg.Artifacts...)
		out = append(out, cp)
	}
	return out
}
