package service

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ninenhan/go-chat/core"
)

const defaultHTTPProxyClientTimeout = 5 * time.Minute

// BuildProxyURL 将代理配置转换为 URL，未配置时返回 nil。
func BuildProxyURL(proxy *core.ProxyConfig) (*url.URL, error) {
	if proxy == nil {
		return nil, nil
	}
	ip := strings.TrimSpace(proxy.IP)
	if ip == "" {
		return nil, errors.New("proxy.ip 不能为空")
	}
	if proxy.Port <= 0 || proxy.Port > 65535 {
		return nil, errors.New("proxy.port 非法")
	}
	scheme := strings.TrimSpace(proxy.Scheme)
	if scheme == "" {
		scheme = "http"
	}
	return &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(ip, strconv.Itoa(proxy.Port)),
	}, nil
}

// NewHTTPClientWithProxy 创建支持代理的 HTTP 客户端。
func NewHTTPClientWithProxy(proxy *core.ProxyConfig, timeout time.Duration) (*http.Client, error) {
	if timeout <= 0 {
		timeout = defaultHTTPProxyClientTimeout
	}
	transport := &http.Transport{}
	if proxy != nil {
		proxyURL, err := BuildProxyURL(proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}

// DoHTTPRequestWithProxy 使用代理配置发送 HTTP 请求。
func DoHTTPRequestWithProxy(
	ctx context.Context,
	method string,
	rawURL string,
	headers map[string][]string,
	body []byte,
	proxy *core.ProxyConfig,
	timeout time.Duration,
) (*http.Response, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("url 不能为空")
	}
	if strings.TrimSpace(method) == "" {
		method = http.MethodPost
	}
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	for k, values := range headers {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	client, err := NewHTTPClientWithProxy(proxy, timeout)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}
