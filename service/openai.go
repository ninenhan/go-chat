package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"feo.vip/chat/core"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
)

const defaultAzureAPIVersion = "2024-06-01"

func NewChatClient(baseURL, apiKey string) *openai.Client {
	apiKey = strings.TrimSpace(apiKey)
	baseURL = strings.TrimSpace(baseURL)

	opts := make([]option.RequestOption, 0, 4)

	// Azure OpenAI 需要额外的 endpoint 与 api-version 处理。
	if isAzureEndpoint(baseURL) {
		endpoint, apiVersion := parseAzureEndpoint(baseURL)
		if apiKey != "" {
			opts = append(opts, azure.WithAPIKey(apiKey))
		}
		opts = append(opts, azure.WithEndpoint(endpoint, apiVersion))
	} else {
		if apiKey != "" {
			opts = append(opts, option.WithAPIKey(apiKey))
		}
		if normalized, ok := normalizeBaseURL(baseURL); ok {
			opts = append(opts, option.WithBaseURL(normalized))
		}
		// 火山 (含豆包) 和 GLM 也是 OpenAI 兼容协议，这里只需提供自定义 BaseURL。
		if isVolcOrDoubao(baseURL) || isGLM(baseURL) {
			// 无需特殊处理，WithAPIKey 已生成 Bearer 头；BaseURL 已被规范化。
		}
	}

	client := openai.NewClient(opts...)
	return &client
}

func isAzureEndpoint(baseURL string) bool {
	return strings.Contains(baseURL, ".openai.azure.com")
}

func isVolcOrDoubao(baseURL string) bool {
	host := parseHost(baseURL)
	return strings.Contains(host, "volces.com") || strings.Contains(host, "volcengineapi.com")
}

func isGLM(baseURL string) bool {
	host := parseHost(baseURL)
	return strings.Contains(host, "bigmodel.cn") || strings.Contains(host, "zhipuai.cn") || strings.Contains(host, "zhipuai.com")
}

func parseAzureEndpoint(raw string) (endpoint, apiVersion string) {
	apiVersion = defaultAzureAPIVersion
	if raw == "" {
		return raw, apiVersion
	}
	u, err := url.Parse(raw)
	if err != nil {
		slog.Warn("无法解析 Azure endpoint，回退默认配置", "baseURL", raw, "err", err)
		return raw, apiVersion
	}
	if version := u.Query().Get("api-version"); version != "" {
		apiVersion = version
	}
	u.RawQuery = ""
	endpoint = strings.TrimSuffix(u.String(), "/")
	return endpoint, apiVersion
}

func normalizeBaseURL(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		slog.Warn("baseURL 无法解析，忽略自定义地址", "baseURL", raw, "err", err)
		return "", false
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	switch u.Path {
	case "", "/":
		u.Path = "/v1/"
	default:
		if !strings.HasSuffix(u.Path, "/") {
			u.Path += "/"
		}
	}
	return u.String(), true
}

func parseHost(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// ChatWithXRequest 基于 XRequest 直接发起 OpenAI 兼容 chat/completions 请求。
// 返回通道：非流式时包含一次结果；流式时逐片推送 ChatGPTStreamResponse。
func ChatWithXRequest(ctx context.Context, xReq *core.XRequest) (chan any, error) {
	if xReq == nil || xReq.Url == "" {
		return nil, errors.New("XRequest 不能为空且需要 url")
	}
	logbts, _ := json.Marshal(&xReq)
	slog.Info("🤖 ChatWithXRequest请求", "payload", string(logbts))
	if ctx == nil {
		ctx = context.Background()
	}
	method := xReq.Method
	if method == "" {
		method = http.MethodPost
	}
	bodyBytes, err := json.Marshal(xReq.Body)
	if err != nil {
		return nil, err
	}
	resp, err := DoHTTPRequestWithProxy(
		ctx,
		method,
		xReq.Url,
		xReq.Headers,
		bodyBytes,
		xReq.Proxy,
		5*time.Minute,
	)
	if err != nil {
		// wrap error to  user
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(b))
	}

	if xReq.Stream {
		ch := make(chan any)
		go func() {
			defer resp.Body.Close()
			defer close(ch)
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}
				if line == "[DONE]" {
					return
				}
				if strings.HasPrefix(line, "data:") {
					line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				}
				if line == "[DONE]" {
					return
				}
				var streamResp core.ChatGPTStreamResponse
				if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
					ch <- err
					return
				}
				if len(streamResp.Choices) == 0 {
					continue
				}
				ch <- streamResp
			}
			if err := scanner.Err(); err != nil {
				ch <- err
			}
		}()
		return ch, nil
	}

	defer resp.Body.Close()
	var result core.ChatGPTResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	ch := make(chan any, 1)
	ch <- result
	close(ch)
	return ch, nil
}

// NewChatXRequest 生成一个基于 OpenAI 兼容接口的 chat/completions 调用请求。
// - baseURL: OpenAI/火山/豆包/GLM/Azure 兼容的根地址
// - apiKey: 认证 key（Azure 会在前置层处理 Api-Key，其他为 Bearer）
// - model/messages/stream: 标准 chat completion 参数
// - extraHeaders: 允许追加或覆盖 header（会在默认头后应用）
// - extraBody: 允许在 body 中追加额外字段（如 temperature/top_p 等）
func NewChatXRequest(
	baseURL, apiKey, model string,
	messages []map[string]string,
	stream bool,
	extraHeaders map[string][]string,
	extraBody map[string]any,
) *core.XRequest {
	normalized, ok := normalizeBaseURL(baseURL)
	if !ok {
		normalized = "https://api.openai.com/v1/"
	}
	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	if apiKey != "" {
		headers["Authorization"] = []string{"Bearer " + apiKey}
	}
	for k, v := range extraHeaders {
		headers[k] = v
	}
	body := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   stream,
	}
	for k, v := range extraBody {
		body[k] = v
	}
	return &core.XRequest{
		Url:     normalized + "chat/completions",
		Method:  "POST",
		Headers: headers,
		Body:    body,
		Stream:  stream,
	}
}
