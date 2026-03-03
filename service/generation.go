package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"feo.vip/chat/core"
	"feo.vip/chat/model"
	"github.com/ninenhan/go-workflow/fn"
)

var (
	generationSlotKeyRule = regexp.MustCompile(`^[a-zA-Z0-9@:_\-\.\$\p{Han}]+$`)
)

// GenerationConfig 控制 service 行为。
type GenerationConfig struct {
	SessionStore         GenerationSessionStore
	EnableQueue          bool
	QueueSize            int
	WorkerCount          int
	ContextLimitTokens   int
	ReservedOutputTokens int
	TaskIDGenerator      func() string
	TokenEstimator       func(text string) int
	Summarizer           GenerationSummarizer
	ChatCaller           GenerationChatCaller
}

// GenerationOption 用于配置 service。
type GenerationOption func(*GenerationConfig)

func WithGenerationSessionStore(store GenerationSessionStore) GenerationOption {
	return func(cfg *GenerationConfig) {
		cfg.SessionStore = store
	}
}

func WithGenerationQueue(enable bool, workerCount, queueSize int) GenerationOption {
	return func(cfg *GenerationConfig) {
		cfg.EnableQueue = enable
		if workerCount > 0 {
			cfg.WorkerCount = workerCount
		}
		if queueSize > 0 {
			cfg.QueueSize = queueSize
		}
	}
}

func WithGenerationTaskIDGenerator(gen func() string) GenerationOption {
	return func(cfg *GenerationConfig) {
		if gen != nil {
			cfg.TaskIDGenerator = gen
		}
	}
}

func WithGenerationContext(limitTokens, reservedOutputTokens int) GenerationOption {
	return func(cfg *GenerationConfig) {
		if limitTokens > 0 {
			cfg.ContextLimitTokens = limitTokens
		}
		if reservedOutputTokens > 0 {
			cfg.ReservedOutputTokens = reservedOutputTokens
		}
	}
}

func WithGenerationTokenEstimator(estimator func(text string) int) GenerationOption {
	return func(cfg *GenerationConfig) {
		if estimator != nil {
			cfg.TokenEstimator = estimator
		}
	}
}

func WithGenerationSummarizer(summarizer GenerationSummarizer) GenerationOption {
	return func(cfg *GenerationConfig) {
		cfg.Summarizer = summarizer
	}
}

func WithGenerationChatCaller(caller GenerationChatCaller) GenerationOption {
	return func(cfg *GenerationConfig) {
		if caller != nil {
			cfg.ChatCaller = caller
		}
	}
}

// GenerationAsyncResult 是 Submit 异步回调结果。
type GenerationAsyncResult struct {
	TaskID   string
	Response *model.GenerationGenerateResponse
	Err      error
}

// GenerationService 暴露 AI 文案核心能力。
type GenerationService interface {
	GetConfig() GenerationConfig
	Close()

	ParseTemplateSlots(template string) ([]model.GenerationSlot, error)
	RenderTemplate(template string, vars map[string]string) (string, error)
	BuildPrompt(req *model.GenerationGenerateRequest) (string, error)

	Generate(ctx context.Context, req *model.GenerationGenerateRequest) (*model.GenerationGenerateResponse, error)
	GenerateStream(ctx context.Context, req *model.GenerationGenerateRequest, onDelta func(delta string) error) (*model.GenerationGenerateResponse, error)
	StartSession(ctx context.Context, req *model.GenerationSessionStartRequest) (*model.GenerationSession, error)
	GetSession(ctx context.Context, sessionID string) (*model.GenerationSession, error)
	ChatSession(ctx context.Context, req *model.GenerationSessionChatRequest, onDelta func(delta string) error) (*model.GenerationSessionChatResponse, error)

	// Submit 提交到异步执行路径：
	// 1) 开启 queue 时进入 worker 队列；
	// 2) 未开启 queue 时直接 goroutine 执行。
	Submit(ctx context.Context, req *model.GenerationGenerateRequest) (<-chan GenerationAsyncResult, error)
}

type queuedTask struct {
	ctx    context.Context
	req    *model.GenerationGenerateRequest
	result chan GenerationAsyncResult
}

// DefaultGenerationService 是默认实现。
type DefaultGenerationService struct {
	cfg          GenerationConfig
	queue        chan queuedTask
	workerCtx    context.Context
	workerCancel context.CancelFunc
	workerOnce   sync.Once
	sessionMu    sync.RWMutex
	sessions     map[string]*model.GenerationSession
}

func defaultGenerationConfig() GenerationConfig {
	return GenerationConfig{
		EnableQueue:          true,
		QueueSize:            128,
		WorkerCount:          2,
		ContextLimitTokens:   32000,
		ReservedOutputTokens: 2048,
		TaskIDGenerator: func() string {
			return fmt.Sprintf("cw_%d", time.Now().UnixNano())
		},
		TokenEstimator: defaultGenerationTokenEstimator,
		ChatCaller:     ChatWithXRequest,
	}
}

func NewGenerationService(opts ...GenerationOption) GenerationService {
	cfg := defaultGenerationConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 128
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 1
	}
	if cfg.TaskIDGenerator == nil {
		cfg.TaskIDGenerator = func() string {
			return fmt.Sprintf("cw_%d", time.Now().UnixNano())
		}
	}
	if cfg.ContextLimitTokens <= 0 {
		cfg.ContextLimitTokens = 32000
	}
	if cfg.ReservedOutputTokens <= 0 {
		cfg.ReservedOutputTokens = 2048
	}
	if cfg.TokenEstimator == nil {
		cfg.TokenEstimator = defaultGenerationTokenEstimator
	}
	if cfg.ChatCaller == nil {
		cfg.ChatCaller = ChatWithXRequest
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &DefaultGenerationService{
		cfg:          cfg,
		workerCtx:    ctx,
		workerCancel: cancel,
		sessions:     make(map[string]*model.GenerationSession),
	}
	if cfg.EnableQueue {
		s.queue = make(chan queuedTask, cfg.QueueSize)
		s.startWorkers()
	}
	return s
}

func (s *DefaultGenerationService) GetConfig() GenerationConfig {
	return s.cfg
}

func (s *DefaultGenerationService) Close() {
	if s.workerCancel != nil {
		s.workerCancel()
	}
}

func (s *DefaultGenerationService) startWorkers() {
	s.workerOnce.Do(func() {
		for i := 0; i < s.cfg.WorkerCount; i++ {
			go s.workerLoop()
		}
	})
}

func (s *DefaultGenerationService) workerLoop() {
	for {
		select {
		case <-s.workerCtx.Done():
			return
		case job := <-s.queue:
			resp, err := s.generate(job.ctx, job.req, false, nil)
			job.result <- GenerationAsyncResult{TaskID: job.req.TaskID, Response: resp, Err: err}
			close(job.result)
		}
	}
}

func (s *DefaultGenerationService) ParseTemplateSlots(template string) ([]model.GenerationSlot, error) {
	template = strings.TrimSpace(template)
	if template == "" {
		return []model.GenerationSlot{}, nil
	}

	parsed, err := fn.ParseTemplate(template)
	if err != nil {
		return nil, err
	}
	slots := make([]model.GenerationSlot, 0, len(parsed))
	keys := make([]string, 0, len(parsed))
	for key := range parsed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, errors.New("template 包含空插槽")
		}
		if !generationSlotKeyRule.MatchString(key) {
			return nil, fmt.Errorf("template 插槽不合法: %s", key)
		}
		slots = append(slots, model.GenerationSlot{
			Key:         key,
			Placeholder: parsed[key].Template,
		})
	}
	return slots, nil
}

func (s *DefaultGenerationService) RenderTemplate(template string, vars map[string]string) (string, error) {
	template = strings.TrimSpace(template)
	if template == "" {
		return "", nil
	}
	parsed, err := fn.ParseTemplate(template)
	if err != nil {
		return "", err
	}
	if len(parsed) == 0 {
		return template, nil
	}
	if vars == nil {
		vars = map[string]string{}
	}

	missing := make([]string, 0)
	for key, needle := range parsed {
		if _, ok := vars[key]; ok {
			continue
		}
		// workflow 模板支持默认值 {{field:default}}，有默认值时不强制外部传入。
		if strings.TrimSpace(needle.DefaultValue) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return "", fmt.Errorf("缺少模板变量: %s", strings.Join(missing, ", "))
	}

	renderModel := make(map[string]any, len(vars))
	for k, v := range vars {
		renderModel[k] = v
	}
	return fn.RenderTemplateWithControl(template, renderModel)
}

func (s *DefaultGenerationService) BuildPrompt(req *model.GenerationGenerateRequest) (string, error) {
	if req == nil {
		return "", errors.New("request 不能为空")
	}
	if strings.TrimSpace(req.Template) != "" {
		return s.RenderTemplate(req.Template, req.TemplateVars)
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return "", errors.New("prompt 不能为空")
	}
	return prompt, nil
}

func (s *DefaultGenerationService) Generate(ctx context.Context, req *model.GenerationGenerateRequest) (*model.GenerationGenerateResponse, error) {
	r := cloneGenerationRequest(req)
	if r.TaskID == "" {
		r.TaskID = s.cfg.TaskIDGenerator()
	}
	return s.generate(ctx, r, false, nil)
}

func (s *DefaultGenerationService) GenerateStream(ctx context.Context, req *model.GenerationGenerateRequest, onDelta func(delta string) error) (*model.GenerationGenerateResponse, error) {
	r := cloneGenerationRequest(req)
	if r.TaskID == "" {
		r.TaskID = s.cfg.TaskIDGenerator()
	}
	return s.generate(ctx, r, true, onDelta)
}

func (s *DefaultGenerationService) Submit(ctx context.Context, req *model.GenerationGenerateRequest) (<-chan GenerationAsyncResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	r := cloneGenerationRequest(req)
	if r.TaskID == "" {
		r.TaskID = s.cfg.TaskIDGenerator()
	}
	resultCh := make(chan GenerationAsyncResult, 1)
	if !s.cfg.EnableQueue {
		go func() {
			resp, err := s.generate(ctx, r, false, nil)
			resultCh <- GenerationAsyncResult{TaskID: r.TaskID, Response: resp, Err: err}
			close(resultCh)
		}()
		return resultCh, nil
	}

	s.startWorkers()
	job := queuedTask{ctx: ctx, req: r, result: resultCh}
	select {
	case <-s.workerCtx.Done():
		return nil, errors.New("generation service 已关闭")
	case <-ctx.Done():
		return nil, ctx.Err()
	case s.queue <- job:
		return resultCh, nil
	}
}

func (s *DefaultGenerationService) generate(
	ctx context.Context,
	req *model.GenerationGenerateRequest,
	stream bool,
	onDelta func(delta string) error,
) (*model.GenerationGenerateResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req == nil {
		return nil, errors.New("request 不能为空")
	}
	if strings.TrimSpace(req.TaskID) == "" {
		req.TaskID = s.cfg.TaskIDGenerator()
	}
	if strings.TrimSpace(req.Model) == "" {
		err := errors.New("model 不能为空")
		return nil, s.failAndMerge(ctx, req.TaskID, err)
	}

	prompt, err := s.BuildPrompt(req)
	if err != nil {
		return nil, s.failAndMerge(ctx, req.TaskID, err)
	}
	if err := s.persistRunning(ctx, req.TaskID, prompt, req.Model); err != nil {
		return nil, err
	}

	messages := make([]map[string]string, 0, len(req.SystemPrompts)+1)
	for _, sp := range req.SystemPrompts {
		sp = strings.TrimSpace(sp)
		if sp == "" {
			continue
		}
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": sp,
		})
	}
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": prompt,
	})

	xReq := NewChatXRequest(
		req.BaseURL,
		req.APIKey,
		req.Model,
		messages,
		stream,
		req.ExtraHeaders,
		req.ExtraBody,
	)
	resultCh, err := s.callChat(ctx, xReq)
	if err != nil {
		return nil, s.failAndMerge(ctx, req.TaskID, err)
	}

	resp := &model.GenerationGenerateResponse{
		TaskID: req.TaskID,
		Status: model.GenerationTaskRunning,
		Prompt: prompt,
	}
	var builder strings.Builder
	chunks := make([]string, 0)
	hasOutput := false

	for item := range resultCh {
		switch v := item.(type) {
		case error:
			return nil, s.failAndMerge(ctx, req.TaskID, v)
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
					return nil, s.failAndMerge(ctx, req.TaskID, err)
				}
			}
		default:
			err := fmt.Errorf("不支持的响应类型: %T", item)
			return nil, s.failAndMerge(ctx, req.TaskID, err)
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
		return nil, s.failAndMerge(ctx, req.TaskID, err)
	}
	if err := s.persistComplete(ctx, req.TaskID, resp.Output); err != nil {
		return nil, err
	}
	resp.Status = model.GenerationTaskCompleted
	return resp, nil
}

func (s *DefaultGenerationService) persistRunning(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (s *DefaultGenerationService) persistComplete(_ context.Context, _ string, _ string) error {
	return nil
}

func (s *DefaultGenerationService) failAndMerge(_ context.Context, _ string, runErr error) error {
	if runErr == nil {
		return nil
	}
	return runErr
}

func cloneGenerationRequest(req *model.GenerationGenerateRequest) *model.GenerationGenerateRequest {
	if req == nil {
		return &model.GenerationGenerateRequest{}
	}
	out := *req
	if req.TemplateVars != nil {
		out.TemplateVars = make(map[string]string, len(req.TemplateVars))
		for k, v := range req.TemplateVars {
			out.TemplateVars[k] = v
		}
	}
	if req.SystemPrompts != nil {
		out.SystemPrompts = append([]string(nil), req.SystemPrompts...)
	}
	if req.ExtraHeaders != nil {
		out.ExtraHeaders = make(map[string][]string, len(req.ExtraHeaders))
		for k, v := range req.ExtraHeaders {
			out.ExtraHeaders[k] = append([]string(nil), v...)
		}
	}
	if req.ExtraBody != nil {
		out.ExtraBody = make(map[string]any, len(req.ExtraBody))
		for k, v := range req.ExtraBody {
			out.ExtraBody[k] = v
		}
	}
	return &out
}

func (s *DefaultGenerationService) callChat(ctx context.Context, xReq *core.XRequest) (chan any, error) {
	caller := s.cfg.ChatCaller
	if caller == nil {
		caller = ChatWithXRequest
	}
	return caller(ctx, xReq)
}

func (s *DefaultGenerationService) estimateTokens(text string) int {
	estimator := s.cfg.TokenEstimator
	if estimator == nil {
		estimator = defaultGenerationTokenEstimator
	}
	n := estimator(text)
	if n < 0 {
		return 0
	}
	return n
}

func defaultGenerationTokenEstimator(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	// 经验值：中英文混合场景按 1 token ~= 4 chars 估算，至少为 1。
	n := len([]rune(trimmed)) / 4
	if n <= 0 {
		return 1
	}
	return n
}
