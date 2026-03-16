package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ninenhan/go-chat/model"
)

type openAIImageResponse struct {
	Created int64 `json:"created,omitempty"`
	Data    []struct {
		URL           string `json:"url,omitempty"`
		B64JSON       string `json:"b64_json,omitempty"`
		RevisedPrompt string `json:"revised_prompt,omitempty"`
	} `json:"data,omitempty"`
}

type imageUploadPayload struct {
	Filename string
	MIMEType string
	Content  []byte
}

// OpenAIImageCaller 是 OpenAI-compatible 图片任务适配器。
// - image_generate -> /images/generations (application/json)
// - image_edit -> /images/edits (multipart/form-data)
func OpenAIImageCaller(ctx context.Context, req *model.GenerationImageRequest) (*model.GenerationImageResponse, error) {
	if req == nil {
		return nil, errors.New("request 不能为空")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, ok := normalizeBaseURL(req.BaseURL)
	if !ok {
		normalized = "https://api.openai.com/v1/"
	}
	taskType := req.TaskType
	if !taskType.IsValid() {
		if len(req.InputImages) > 0 || req.MaskImage != nil {
			taskType = model.GenerationTaskTypeImageEdit
		} else {
			taskType = model.GenerationTaskTypeImageGenerate
		}
	}

	switch taskType {
	case model.GenerationTaskTypeImageGenerate:
		return openAIImageGenerate(ctx, normalized, req)
	case model.GenerationTaskTypeImageEdit:
		return openAIImageEdit(ctx, normalized, req)
	default:
		return nil, fmt.Errorf("OpenAIImageCaller 不支持任务类型: %s", taskType)
	}
}

func openAIImageGenerate(ctx context.Context, normalizedBaseURL string, req *model.GenerationImageRequest) (*model.GenerationImageResponse, error) {
	inputImages := normalizeImageInputImages(req)
	body := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
	}
	if req.N > 0 {
		body["n"] = req.N
	}
	if v := strings.TrimSpace(req.Size); v != "" {
		body["size"] = v
	}
	if v := strings.TrimSpace(req.Quality); v != "" {
		body["quality"] = v
	}
	if v := strings.TrimSpace(req.Style); v != "" {
		body["style"] = v
	}
	if v := strings.TrimSpace(req.NegativePrompt); v != "" {
		body["negative_prompt"] = v
	}
	if len(inputImages) == 1 {
		body["image"] = strings.TrimSpace(inputImages[0].URI)
	}
	if len(inputImages) > 1 {
		uris := make([]string, 0, len(inputImages))
		for _, item := range inputImages {
			if uri := strings.TrimSpace(item.URI); uri != "" {
				uris = append(uris, uri)
			}
		}
		if len(uris) == 1 {
			body["image"] = uris[0]
		} else if len(uris) > 1 {
			body["images"] = uris
		}
	}
	if v := strings.TrimSpace(req.OutputFormat); v != "" {
		body["output_format"] = v
	}
	if req.Watermark != nil {
		body["watermark"] = *req.Watermark
	}
	for k, v := range req.ExtraBody {
		body[k] = v
	}

	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	if req.APIKey != "" {
		headers["Authorization"] = []string{"Bearer " + req.APIKey}
	}
	for k, v := range req.ExtraHeaders {
		headers[k] = append([]string(nil), v...)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := DoHTTPRequestWithProxy(
		ctx,
		http.MethodPost,
		normalizedBaseURL+"images/generations",
		headers,
		payload,
		nil,
		5*time.Minute,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeOpenAIImageResponse(resp, req, model.GenerationTaskTypeImageGenerate)
}

func openAIImageEdit(ctx context.Context, normalizedBaseURL string, req *model.GenerationImageRequest) (*model.GenerationImageResponse, error) {
	inputImages := normalizeImageInputImages(req)
	if len(inputImages) == 0 {
		return nil, errors.New("image_edit 至少需要一张输入图片")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", req.Model); err != nil {
		return nil, err
	}
	if err := writer.WriteField("prompt", req.Prompt); err != nil {
		return nil, err
	}
	if req.N > 0 {
		if err := writer.WriteField("n", strconv.Itoa(req.N)); err != nil {
			return nil, err
		}
	}
	if v := strings.TrimSpace(req.Size); v != "" {
		if err := writer.WriteField("size", v); err != nil {
			return nil, err
		}
	}
	if v := strings.TrimSpace(req.Quality); v != "" {
		if err := writer.WriteField("quality", v); err != nil {
			return nil, err
		}
	}
	if v := strings.TrimSpace(req.Style); v != "" {
		if err := writer.WriteField("style", v); err != nil {
			return nil, err
		}
	}
	if v := strings.TrimSpace(req.NegativePrompt); v != "" {
		if err := writer.WriteField("negative_prompt", v); err != nil {
			return nil, err
		}
	}
	for k, v := range req.ExtraBody {
		if err := writeMultipartValue(writer, k, v); err != nil {
			return nil, err
		}
	}
	for _, item := range inputImages {
		if err := writeAttachmentFile(writer, "image", item); err != nil {
			return nil, err
		}
	}
	if req.MaskImage != nil {
		if err := writeAttachmentFile(writer, "mask", *req.MaskImage); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	headers := map[string][]string{
		"Content-Type": {writer.FormDataContentType()},
	}
	if req.APIKey != "" {
		headers["Authorization"] = []string{"Bearer " + req.APIKey}
	}
	for k, v := range req.ExtraHeaders {
		headers[k] = append([]string(nil), v...)
	}

	resp, err := DoHTTPRequestWithProxy(
		ctx,
		http.MethodPost,
		normalizedBaseURL+"images/edits",
		headers,
		body.Bytes(),
		nil,
		10*time.Minute,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeOpenAIImageResponse(resp, req, model.GenerationTaskTypeImageEdit)
}

func decodeOpenAIImageResponse(resp *http.Response, req *model.GenerationImageRequest, taskType model.GenerationTaskType) (*model.GenerationImageResponse, error) {
	if resp == nil {
		return nil, errors.New("response 不能为空")
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(b))
	}
	var raw openAIImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	artifacts := make([]model.GenerationArtifact, 0, len(raw.Data))
	for idx, item := range raw.Data {
		artifact := model.GenerationArtifact{
			ID:   fmt.Sprintf("img_%d", idx+1),
			Kind: "image",
		}
		switch {
		case strings.TrimSpace(item.URL) != "":
			artifact.URI = strings.TrimSpace(item.URL)
		case strings.TrimSpace(item.B64JSON) != "":
			artifact.URI = "data:image/png;base64," + strings.TrimSpace(item.B64JSON)
			artifact.MimeType = "image/png"
		}
		if artifact.URI == "" {
			continue
		}
		artifacts = append(artifacts, artifact)
	}
	return &model.GenerationImageResponse{
		TaskID:    req.TaskID,
		TaskType:  taskType,
		Status:    model.GenerationTaskCompleted,
		Prompt:    req.Prompt,
		Output:    defaultImageOutputText(artifacts),
		Artifacts: artifacts,
		Raw:       raw,
	}, nil
}

func writeMultipartValue(writer *multipart.Writer, key string, value any) error {
	key = strings.TrimSpace(key)
	if key == "" || value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		return writer.WriteField(key, v)
	case fmt.Stringer:
		return writer.WriteField(key, v.String())
	case int:
		return writer.WriteField(key, strconv.Itoa(v))
	case int8:
		return writer.WriteField(key, strconv.FormatInt(int64(v), 10))
	case int16:
		return writer.WriteField(key, strconv.FormatInt(int64(v), 10))
	case int32:
		return writer.WriteField(key, strconv.FormatInt(int64(v), 10))
	case int64:
		return writer.WriteField(key, strconv.FormatInt(v, 10))
	case uint:
		return writer.WriteField(key, strconv.FormatUint(uint64(v), 10))
	case uint8:
		return writer.WriteField(key, strconv.FormatUint(uint64(v), 10))
	case uint16:
		return writer.WriteField(key, strconv.FormatUint(uint64(v), 10))
	case uint32:
		return writer.WriteField(key, strconv.FormatUint(uint64(v), 10))
	case uint64:
		return writer.WriteField(key, strconv.FormatUint(v, 10))
	case float32:
		return writer.WriteField(key, strconv.FormatFloat(float64(v), 'f', -1, 32))
	case float64:
		return writer.WriteField(key, strconv.FormatFloat(v, 'f', -1, 64))
	case bool:
		return writer.WriteField(key, strconv.FormatBool(v))
	default:
		bs, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return writer.WriteField(key, string(bs))
	}
}

func writeAttachmentFile(writer *multipart.Writer, field string, item model.GenerationAttachment) error {
	payload, err := loadAttachmentPayload(item)
	if err != nil {
		return err
	}
	part, err := writer.CreateFormFile(field, payload.Filename)
	if err != nil {
		return err
	}
	_, err = part.Write(payload.Content)
	return err
}

func loadAttachmentPayload(item model.GenerationAttachment) (*imageUploadPayload, error) {
	uri := strings.TrimSpace(item.URI)
	if uri == "" {
		return nil, errors.New("attachment uri 不能为空")
	}
	if strings.HasPrefix(uri, "data:") {
		return parseDataURI(item)
	}
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return nil, errors.New("当前 image_edit 仅支持本地文件或 data URI，不支持远程 URL")
	}
	bs, err := os.ReadFile(uri)
	if err != nil {
		return nil, err
	}
	filename := strings.TrimSpace(item.Name)
	if filename == "" {
		filename = filepath.Base(uri)
	}
	mimeType := strings.TrimSpace(item.MimeType)
	if mimeType == "" {
		mimeType = mimeFromFilename(filename)
	}
	return &imageUploadPayload{
		Filename: filename,
		MIMEType: mimeType,
		Content:  bs,
	}, nil
}

func parseDataURI(item model.GenerationAttachment) (*imageUploadPayload, error) {
	uri := strings.TrimSpace(item.URI)
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return nil, errors.New("非法 data URI")
	}
	meta := uri[len("data:"):comma]
	data := uri[comma+1:]
	if !strings.Contains(meta, ";base64") {
		return nil, errors.New("当前仅支持 base64 data URI")
	}
	mimeType := meta
	if semi := strings.IndexByte(meta, ';'); semi >= 0 {
		mimeType = meta[:semi]
	}
	content, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	filename := strings.TrimSpace(item.Name)
	if filename == "" {
		filename = "upload" + extFromMIME(mimeType)
	}
	return &imageUploadPayload{
		Filename: filename,
		MIMEType: mimeType,
		Content:  content,
	}, nil
}

func mimeFromFilename(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func extFromMIME(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}
