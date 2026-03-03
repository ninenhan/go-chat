package core

import (
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
)

// SSEProxy SSE 代理处理函数
func SSEProxy(c *gin.Context, resp *http.Response) {
	// 复制后端响应头到客户端
	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	// 设置当前响应的头部
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// 流式传输响应内容
	// 复制响应体并流式传输
	c.Writer.Flush()
	// Create a buffer for streaming
	buffer := make([]byte, 1024*8)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "Streaming unsupported!")
		return
	}
	// 这里不能简单的copy，需要手动buffer
	// Stream data manually
	for {
		// Read from the backend response body
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			// Write to the client
			_, _ = c.Writer.Write(buffer[:n])
			flusher.Flush() // Ensure the data is sent immediately
		}
		if err != nil {
			if err != io.EOF {
				log.Println("Error while streaming:", err)
			}
			break
		}
	}

}
