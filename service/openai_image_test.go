package service

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ninenhan/go-chat/model"
)

func TestOpenAIImageCallerGenerate(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		gotAuth = r.Header.Get("Authorization")

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if body["model"] != "gpt-image-1" {
			t.Fatalf("unexpected model: %+v", body["model"])
		}
		if body["prompt"] != "draw a cat" {
			t.Fatalf("unexpected prompt: %+v", body["prompt"])
		}
		if body["image"] != "https://img.local/input.png" {
			t.Fatalf("unexpected image: %+v", body["image"])
		}
		if body["output_format"] != "png" {
			t.Fatalf("unexpected output_format: %+v", body["output_format"])
		}
		if body["watermark"] != false {
			t.Fatalf("unexpected watermark: %+v", body["watermark"])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 123,
			"data": []map[string]any{
				{"url": "https://img.local/cat.png"},
			},
		})
	}))
	defer server.Close()
	watermark := false

	resp, err := OpenAIImageCaller(context.Background(), &model.GenerationImageRequest{
		TaskID:       "t1",
		TaskType:     model.GenerationTaskTypeImageGenerate,
		BaseURL:      server.URL,
		APIKey:       "secret",
		Model:        "gpt-image-1",
		Prompt:       "draw a cat",
		Image:        "https://img.local/input.png",
		Size:         "1024x1024",
		OutputFormat: "png",
		Watermark:    &watermark,
	})
	if err != nil {
		t.Fatalf("OpenAIImageCaller error: %v", err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("unexpected auth header: %s", gotAuth)
	}
	if resp.TaskType != model.GenerationTaskTypeImageGenerate {
		t.Fatalf("unexpected task type: %s", resp.TaskType)
	}
	if len(resp.Artifacts) != 1 || resp.Artifacts[0].URI != "https://img.local/cat.png" {
		t.Fatalf("unexpected artifacts: %+v", resp.Artifacts)
	}
}

func TestOpenAIImageCallerEdit(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "input.png")
	if err := os.WriteFile(imagePath, []byte("fake-png"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm error: %v", err)
		}
		if r.FormValue("model") != "gpt-image-1" {
			t.Fatalf("unexpected model: %s", r.FormValue("model"))
		}
		if r.FormValue("prompt") != "make it cyberpunk" {
			t.Fatalf("unexpected prompt: %s", r.FormValue("prompt"))
		}
		files := r.MultipartForm.File["image"]
		if len(files) != 1 {
			t.Fatalf("unexpected image file count: %d", len(files))
		}
		fh := files[0]
		if fh.Filename != "input.png" {
			t.Fatalf("unexpected filename: %s", fh.Filename)
		}
		file, err := fh.Open()
		if err != nil {
			t.Fatalf("Open file error: %v", err)
		}
		defer file.Close()
		bs, _ := io.ReadAll(file)
		if string(bs) != "fake-png" {
			t.Fatalf("unexpected file content: %s", string(bs))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"b64_json": "ZmFrZS1pbWFnZQ=="},
			},
		})
	}))
	defer server.Close()

	resp, err := OpenAIImageCaller(context.Background(), &model.GenerationImageRequest{
		TaskID:   "t2",
		TaskType: model.GenerationTaskTypeImageEdit,
		BaseURL:  server.URL,
		APIKey:   "secret",
		Model:    "gpt-image-1",
		Prompt:   "make it cyberpunk",
		InputImages: []model.GenerationAttachment{
			{URI: imagePath},
		},
	})
	if err != nil {
		t.Fatalf("OpenAIImageCaller error: %v", err)
	}
	if len(resp.Artifacts) != 1 {
		t.Fatalf("unexpected artifacts: %+v", resp.Artifacts)
	}
	if !strings.HasPrefix(resp.Artifacts[0].URI, "data:image/png;base64,") {
		t.Fatalf("unexpected artifact uri: %s", resp.Artifacts[0].URI)
	}
}

func TestOpenAIImageCallerEditRejectsRemoteURL(t *testing.T) {
	_, err := OpenAIImageCaller(context.Background(), &model.GenerationImageRequest{
		TaskID:   "t3",
		TaskType: model.GenerationTaskTypeImageEdit,
		BaseURL:  "https://api.openai.com",
		Model:    "gpt-image-1",
		Prompt:   "edit",
		InputImages: []model.GenerationAttachment{
			{URI: "https://img.local/input.png"},
		},
	})
	if err == nil {
		t.Fatal("expected error for remote URL input image")
	}
}

func TestWriteMultipartValue(t *testing.T) {
	var body strings.Builder
	writer := multipart.NewWriter(&body)
	if err := writeMultipartValue(writer, "count", 2); err != nil {
		t.Fatalf("writeMultipartValue error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close error: %v", err)
	}
	if !strings.Contains(body.String(), `name="count"`) || !strings.Contains(body.String(), "\r\n2\r\n") {
		t.Fatalf("unexpected multipart body: %s", body.String())
	}
}
