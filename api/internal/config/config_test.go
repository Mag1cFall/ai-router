package config_test

import (
	"os"
	"testing"

	"github.com/Mag1cFall/ai-router/api/internal/config"
)

const validYAML = `
providers:
  - name: openai
    protocol: openai
    endpoint: https://api.openai.com/v1
    api_key: sk-test-openai

  - name: claude
    protocol: claude
    endpoint: https://api.anthropic.com
    api_key: sk-ant-test

  - name: gemini
    protocol: gemini
    endpoint: https://generativelanguage.googleapis.com/v1beta
    api_key: AIza-test

routes:
  - match_model: "gpt-*"
    provider: openai
  - match_model: "claude-*"
    provider: claude
  - match_model: "gemini-*"
    provider: gemini

server:
  port: 8446
  log_level: info
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return f.Name()
}

func TestLoadConfig_Valid(t *testing.T) {
	path := writeTempConfig(t, validYAML)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(cfg.Providers))
	}
	if cfg.Server.Port != 8446 {
		t.Errorf("expected port 8446, got %d", cfg.Server.Port)
	}
}

func TestLoadConfig_ProviderFields(t *testing.T) {
	path := writeTempConfig(t, validYAML)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	p := cfg.Providers[0]
	if p.Name != "openai" {
		t.Errorf("expected name=openai, got %q", p.Name)
	}
	if p.Protocol != config.ProtocolOpenAI {
		t.Errorf("expected protocol=openai, got %v", p.Protocol)
	}
	if p.Endpoint != "https://api.openai.com/v1" {
		t.Errorf("unexpected endpoint: %q", p.Endpoint)
	}
	if p.APIKey != "sk-test-openai" {
		t.Errorf("unexpected api_key")
	}
}

func TestLoadConfig_Routes(t *testing.T) {
	path := writeTempConfig(t, validYAML)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(cfg.Routes) != 3 {
		t.Errorf("expected 3 routes, got %d", len(cfg.Routes))
	}
	if cfg.Routes[0].MatchModel != "gpt-*" {
		t.Error("expected first route match_model=gpt-*")
	}
	if cfg.Routes[0].Provider != "openai" {
		t.Error("expected first route provider=openai")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	path := writeTempConfig(t, "{ invalid yaml [[[")
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid yaml")
	}
}

func TestLoadConfig_MissingEndpoint(t *testing.T) {
	yaml := `
providers:
  - name: openai
    protocol: openai
`
	path := writeTempConfig(t, yaml)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected validation error when endpoint missing")
	}
}

func TestLoadConfig_MissingName(t *testing.T) {
	yaml := `
providers:
  - protocol: openai
    endpoint: https://api.openai.com/v1
`
	path := writeTempConfig(t, yaml)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected validation error when name missing")
	}
}

func TestLoadConfig_MissingProtocol(t *testing.T) {
	yaml := `
providers:
  - name: test
    endpoint: https://api.openai.com/v1
`
	path := writeTempConfig(t, yaml)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected validation error when protocol missing")
	}
}

func TestLoadConfig_RoutePointsToMissingProvider(t *testing.T) {
	yaml := `
providers:
  - name: openai
    protocol: openai
    endpoint: https://api.openai.com/v1

routes:
  - match_model: "gpt-*"
    provider: nonexistent
`
	path := writeTempConfig(t, yaml)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected validation error when route references nonexistent provider")
	}
}

func TestResolveProvider_GlobMatch(t *testing.T) {
	path := writeTempConfig(t, validYAML)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	cases := []struct {
		model    string
		wantName string
	}{
		{"gpt-5.4", "openai"},
		{"gpt-5.3-codex", "openai"},
		{"gpt-5.2", "openai"},
		{"gpt-4o", "openai"},
		{"gpt-4-turbo", "openai"},

		{"claude-sonnet-4-6", "claude"},
		{"claude-sonnet-4-6-thinking", "claude"},
		{"claude-opus-4-5", "claude"},
		{"claude-opus-4", "claude"},
		{"claude-3-5-sonnet", "claude"},

		{"gemini-3-pro-preview", "gemini"},
		{"gemini-3.1-pro-preview", "gemini"},
		{"gemini-3-flash-preview", "gemini"},
		{"gemini-2.5-pro", "gemini"},
		{"gemini-2.0-flash", "gemini"},
	}
	for _, c := range cases {
		p, err := cfg.ResolveProvider(c.model)
		if err != nil {
			t.Errorf("ResolveProvider(%q) error: %v", c.model, err)
			continue
		}
		if p.Name != c.wantName {
			t.Errorf("ResolveProvider(%q) = %q, want %q", c.model, p.Name, c.wantName)
		}
	}
}

func TestResolveProvider_NoMatch(t *testing.T) {
	path := writeTempConfig(t, validYAML)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	_, err = cfg.ResolveProvider("unknown-model-xyz")
	if err == nil {
		t.Error("expected error for unmatched model")
	}
}

func TestResolveProvider_FirstMatchWins(t *testing.T) {
	yaml := `
providers:
  - name: special
    protocol: openai
    endpoint: https://special.api.com/v1
  - name: general
    protocol: openai
    endpoint: https://api.openai.com/v1

routes:
  - match_model: "gpt-5*"
    provider: special
  - match_model: "gpt-*"
    provider: general
`
	path := writeTempConfig(t, yaml)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	p, err := cfg.ResolveProvider("gpt-5.4")
	if err != nil {
		t.Fatalf("ResolveProvider error: %v", err)
	}
	if p.Name != "special" {
		t.Errorf("expected first match (special), got %q", p.Name)
	}

	p, err = cfg.ResolveProvider("gpt-4o")
	if err != nil {
		t.Fatalf("ResolveProvider error: %v", err)
	}
	if p.Name != "general" {
		t.Errorf("expected fallback match (general), got %q", p.Name)
	}
}
