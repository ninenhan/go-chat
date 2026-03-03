package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slog"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ServerSentEvent struct {
	Id    string `json:"id,omitempty"`
	Event string `json:"event,omitempty"`
	Data  string `json:"data,omitempty"`
}

func SSEStandardMessage(event *ServerSentEvent) string {
	return fmt.Sprintf(
		"id: %s\n"+
			"event: %s\n"+
			"data: %s\n\n",
		event.Id,
		event.Event,
		event.Data)
}

type XRequest struct {
	Id        string              `json:"id,omitempty"`
	TopicId   string              `json:"topic_id,omitempty"`
	Url       string              `json:"url,omitempty"`
	Method    string              `json:"method,omitempty"`
	Headers   map[string][]string `json:"headers,omitempty"`
	Body      any                 `json:"body,omitempty"`
	Proxy     *ProxyConfig        `json:"proxy,omitempty"`
	TraceType string              `json:"traceType,omitempty"`
	Stream    bool                `json:"stream,omitempty"`
}

type ProxyConfig struct {
	IP     string `json:"ip,omitempty"`
	Port   int    `json:"port,omitempty"`
	Scheme string `json:"scheme,omitempty"` // default: http
}

// completion_tokens_details

type CompletionTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

type CachedTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type Usage struct {
	PromptTokens            int                     `json:"prompt_tokens,omitempty"`
	CompletionTokens        int                     `json:"completion_tokens,omitempty"`
	TotalTokens             int                     `json:"total_tokens,omitempty"`
	CompletionTokensDetails CompletionTokensDetails `json:"completion_tokens_details,omitempty"`
	CachedTokensDetails     CachedTokensDetails     `json:"cached_tokens"`
}

// ChatGPTRequest ChatGPT API 请求的结构体
type ChatGPTRequest struct {
	Model       string              `json:"model,omitempty"`
	Messages    []map[string]string `json:"messages,omitempty"`
	Stream      *bool               `json:"stream,omitempty"`
	MaxTokens   *int                `json:"max_tokens,omitempty"`
	Temperature *float32            `json:"temperature,omitempty"`
	TopP        *float32            `json:"top_p,omitempty"`
}

type LLMMessage struct {
	Role             string `json:"role,omitempty"`
	Content          string `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"` //deepThinking
	Reasoning        string `json:"reasoning,omitempty"`         // 兼容部分模型返回字段
}

// ChatGPTResponse OpenAI API 响应的结构体（非流式）
type ChatGPTResponse struct {
	ID      string `json:"id,omitempty"`
	Object  string `json:"object,omitempty"`
	Model   string `json:"model,omitempty"`
	Usage   Usage  `json:"usage,omitempty"`
	Choices []struct {
		Index        int        `json:"index,omitempty"`
		Message      LLMMessage `json:"message,omitempty"`
		FinishReason string     `json:"finish_reason,omitempty"`
	} `json:"choices,omitempty"`
}

func (c *ChatGPTResponse) GetResponse() string {
	var results []string
	for _, choice := range c.Choices {
		results = append(results, choice.Message.Content)
	}
	return strings.Join(results, "\n")
}

func (c *ChatGPTResponse) GetDeltaResponse() []LLMMessage {
	var results []LLMMessage
	for _, choice := range c.Choices {
		results = append(results, choice.Message)
	}
	return results
}

// ChatGPTStreamResponse OpenAI API 流式响应的结构体
type ChatGPTStreamResponse struct {
	ID      string `json:"id,omitempty"`
	Object  string `json:"object,omitempty"`
	Model   string `json:"model,omitempty"`
	Usage   *Usage `json:"usage,omitempty"`
	Choices []struct {
		Delta        LLMMessage `json:"delta,omitempty"`
		FinishReason string     `json:"finish_reason,omitempty"`
	} `json:"choices,omitempty"`
}

func (c *ChatGPTStreamResponse) GetResponse() string {
	var results []string
	for _, choice := range c.Choices {
		results = append(results, choice.Delta.Content)
	}
	return strings.Join(results, "")
}

func (c *ChatGPTStreamResponse) GetDeltaResponse() []LLMMessage {
	var results []LLMMessage
	for _, choice := range c.Choices {
		results = append(results, choice.Delta)
	}
	return results
}

type ProxyRequestHandler struct {
	Context *gin.Context
	Request *XRequest
}

// HandlerWithChannel CallChatGPT 调用 OpenAI 的 ChatGPT 接口
func (h *ProxyRequestHandler) HandlerWithChannel(ch chan<- any) error {
	xRequest := h.Request
	// 序列化请求体
	body, err := json.Marshal(xRequest.Body)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %v", err)
	}
	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(context.Background(), xRequest.Method, xRequest.Url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for key, values := range xRequest.Headers {
		req.Header.Set(key, strings.Join(values, ","))
	}
	// 创建 HTTP 客户端并发送请求
	client := &http.Client{
		Timeout: 5 * time.Minute, // 设置超时时间
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error("关闭 Body 失败", err)
		}
	}(resp.Body)

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body) // 读取错误信息
		defer func() {
			close(ch) // 确保只有在流数据处理完后才关闭 Channel
		}()
		ch <- fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
		return fmt.Errorf("请求失败，状态码: %d，响应: %s", resp.StatusCode, string(bodyBytes))
	}
	var timeout time.Duration
	if xRequest.Stream {
		// 假设基于数据量或其他逻辑来计算超时时间
		timeout = 10 * time.Minute
	} else {
		timeout = 3 * time.Minute // 非流式响应可以使用较短的超时
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// 处理 Stream 和非 Stream 两种模式
	if xRequest.Stream {
		// Stream 模式：逐行读取数据流
		return handleStreamResponse(ctx, resp, ch)
	} else {
		// 非 Stream 模式：直接解析完整的 JSON 响应
		return handleNonStreamResponse(ctx, resp, ch)
	}
}

// handleStreamResponse 处理流式响应
func handleStreamResponse(ctx context.Context, resp *http.Response, ch chan<- any) error {
	defer close(ch) // 确保只有在流数据处理完后才关闭 Channel
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		// 将字节数据转为字符串
		if line == "[DONE]" {
			break
		}
		// 处理以 "data:" 开头的数据
		if len(line) > 5 && line[:5] == "data:" {
			line = line[5:] // 去掉 "data: " 前缀
			line = strings.TrimSpace(line)
			if line == "[DONE]" {
				break
			}
			//slog.Info("line", line)
			var streamResp ChatGPTStreamResponse
			if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
				return fmt.Errorf("解析流数据失败: %w", err)
			}
			// 将解析后的数据发送到通道
			// 仅在 choices 为空时跳过，避免吞掉 role-only / reasoning-only 的早期分片。
			if len(streamResp.Choices) == 0 {
				continue
			}
			select {
			case ch <- streamResp:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return nil
}

// 处理非流式响应
func handleNonStreamResponse(ctx context.Context, resp *http.Response, ch chan<- any) error {
	defer func() {
		close(ch) // 确保只有在流数据处理完后才关闭 Channel
	}()
	var response ChatGPTResponse
	err := json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return fmt.Errorf("解析非流式响应失败: %v", err)
	}
	//// 获取所有的内容
	//var results []string
	//for _, choice := range response.Choices {
	//	results = append(results, choice.Message.Content)
	//}
	select {
	case ch <- response: // 将内容发送到 Channel
	case <-ctx.Done(): // 如果 context 被取消，则退出
		return ctx.Err()
	}
	return nil
}

func (h *ProxyRequestHandler) HandleDualResponse(isStream bool, ch chan any, onResult func(res any)) {
	c := h.Context
	var once sync.Once
	for message := range ch {
		_, isErr := message.(error)
		if isErr {
			break
		}
		once.Do(func() {
			if isStream {
				c.Writer.Header().Set("Content-Type", "text/event-stream")
				c.Writer.Header().Set("Cache-Control", "no-cache")
				c.Writer.Header().Set("Connection", "keep-alive")
				c.Writer.Flush()
				notify := c.Writer.CloseNotify()
				go func() {
					<-notify
					slog.Info("Client disconnected")
					close(ch)
				}()
			}
		})

		if isStream {
			bs, _ := json.Marshal(message)
			msg := SSEStandardMessage(&ServerSentEvent{
				Data:  string(bs),
				Event: "data",
				Id:    "0",
			})
			_, err := fmt.Fprint(c.Writer, msg)
			if err != nil {
				slog.Error("写入失败", "err", err)
				return
			}
			c.Writer.Flush()
		} else {
			onResult(message)
			break
		}
	}
}

func (h *ProxyRequestHandler) HandleReadableStream(isStream bool, ch chan any, onResult func(res any)) {
	c := h.Context
	var once sync.Once
	var result []any
	var flusher http.Flusher
	var writeMu sync.Mutex
	done := make(chan struct{})
	defer close(done)
	for message := range ch {
		// 如果是 error，就退出循环
		if _, isErr := message.(error); isErr {
			return
		}
		// 首次写响应 Header
		// 写 header 只能执行一次
		once.Do(func() {
			if !isStream {
				return
			}
			// headers
			// 走 chunk 流输出，避免被中间件/代理合并或压缩
			c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
			c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
			c.Writer.Header().Set("X-Accel-Buffering", "no")
			c.Writer.Header().Set("Content-Encoding", "identity")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.Header().Set("Transfer-Encoding", "chunked")
			// 必须执行，强制写 header，避免被 Gin 缓冲
			c.Writer.WriteHeaderNow()
			fmt.Printf("writer type = %T\n", c.Writer)
			// flusher
			var ok bool
			flusher, ok = c.Writer.(http.Flusher)
			if !ok {
				slog.Error("flusher not supported")
				close(ch)
				return
			}
			flusher.Flush()

			// 心跳 ticker
			ticker := time.NewTicker(15 * time.Second)

			// 最大存活时间
			maxLifetime := time.NewTimer(30 * time.Minute)

			// 客户端断开检测
			closeNotify := c.Writer.CloseNotify()

			// 后台 goroutine 管控连接生命周期
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Warn("panic in heartbeat loop", "panic", r)
					}
				}()
				defer ticker.Stop()
				defer maxLifetime.Stop()

				for {
					select {
					case <-done:
						return

					// 心跳
					case <-ticker.C:
						// *** 在每次写之前确认连接是否还在 ***
						select {
						case <-closeNotify:
							slog.Info("client disconnected (heartbeat loop)")
							return
						default:
						}

						writeMu.Lock()
						_, err := c.Writer.Write([]byte(strings.Repeat(" ", 1024) + "\n"))
						if err != nil {
							writeMu.Unlock()
							slog.Warn("heartbeat write failed", "err", err)
							return
						}
						// flush
						func() {
							defer func() {
								if r := recover(); r != nil {
									slog.Warn("panic in flusher during heartbeat", "panic", r)
								}
							}()
							flusher.Flush()
						}()
						writeMu.Unlock()

					// 超时
					case <-maxLifetime.C:
						slog.Info("max lifetime reached, closing stream")

						writeMu.Lock()
						_, err := c.Writer.Write([]byte(": end\n"))
						if err != nil {
							writeMu.Unlock()
							close(ch)
							return
						}
						flusher.Flush()
						writeMu.Unlock()
						close(ch)
						return

					// 客户端断开
					case <-closeNotify:
						slog.Info("stream client disconnected")
						close(ch)
						return
					}
				}
			}()
		})
		if isStream {
			// Marshal 成 JSON，但作为普通 chunk 发送（非 SSE）
			bs, _ := json.Marshal(message)

			writeMu.Lock()
			_, err := fmt.Fprintf(c.Writer, "%s\n", bs)
			if err != nil {
				writeMu.Unlock()
				slog.Error("写 chunk 失败", "err", err)
				return
			}

			// 强制刷新 chunk 输出
			if flusher != nil {
				flusher.Flush()
				slog.Info("HandleReadableStream message")
			}
			writeMu.Unlock()
			continue
		}
		result = append(result, message)
	}
	if !isStream {
		onResult(result)
	}
	close(done)
}
