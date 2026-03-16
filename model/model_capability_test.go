package model

import "testing"

func TestParseModelBaseConfigDefaultsToTextChat(t *testing.T) {
	cfg, err := ParseModelBaseConfig(nil)
	if err != nil {
		t.Fatalf("ParseModelBaseConfig error: %v", err)
	}
	if !cfg.Capabilities.TextChat {
		t.Fatal("expected default textChat capability to be true")
	}
	if cfg.Capabilities.ImageGenerate || cfg.Capabilities.ImageEdit {
		t.Fatal("expected default image capabilities to be false")
	}
}

func TestGblModelSupportsTaskType(t *testing.T) {
	item := &GblModel{
		Code: "flux-1",
		BaseConfig: MustJSON(ModelBaseConfig{
			Provider:        "replicate",
			DefaultTaskType: GenerationTaskTypeImageGenerate,
			Capabilities: ModelCapabilities{
				ImageGenerate: true,
				ImageEdit:     true,
			},
		}),
	}
	if item.SupportsTaskType(GenerationTaskTypeTextChat) {
		t.Fatal("expected text_chat to be unsupported")
	}
	if !item.SupportsTaskType(GenerationTaskTypeImageGenerate) {
		t.Fatal("expected image_generate to be supported")
	}
	if !item.SupportsTaskType(GenerationTaskTypeImageEdit) {
		t.Fatal("expected image_edit to be supported")
	}
}

func TestGblEndpointParseLimitsConfig(t *testing.T) {
	item := &GblEndpoint{
		ModelCode: "gpt-image-1",
		Limits: MustJSON(ModelLimitsConfig{
			MaxImageCount: 4,
			ImageSizes:    []string{"1024x1024", "1536x1024"},
		}),
	}
	cfg, err := item.ParseLimitsConfig()
	if err != nil {
		t.Fatalf("ParseLimitsConfig error: %v", err)
	}
	if cfg.MaxImageCount != 4 {
		t.Fatalf("unexpected MaxImageCount: %d", cfg.MaxImageCount)
	}
	if len(cfg.ImageSizes) != 2 {
		t.Fatalf("unexpected ImageSizes: %+v", cfg.ImageSizes)
	}
}
