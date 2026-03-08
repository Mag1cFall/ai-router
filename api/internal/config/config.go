package config

import (
	"fmt"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type ProviderProtocol string

const (
	ProtocolUnknown ProviderProtocol = "unknown"
	ProtocolOpenAI  ProviderProtocol = "openai"
	ProtocolClaude  ProviderProtocol = "claude"
	ProtocolGemini  ProviderProtocol = "gemini"
)

type Provider struct {
	Name     string           `yaml:"name" json:"name"`
	Protocol ProviderProtocol `yaml:"protocol" json:"protocol"`
	Endpoint string           `yaml:"endpoint" json:"endpoint"`
	APIKey   string           `yaml:"api_key" json:"-"`
}

type Route struct {
	MatchModel string `yaml:"match_model" json:"match_model"`
	Provider   string `yaml:"provider" json:"provider"`
}

type Server struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	LogLevel string `yaml:"log_level" json:"log_level"`
}

type Config struct {
	Providers []Provider `yaml:"providers"`
	Routes    []Route    `yaml:"routes"`
	Server    Server     `yaml:"server"`

	providerIndex map[string]Provider `yaml:"-"`
}

func (p *ProviderProtocol) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	s := strings.TrimSpace(strings.ToLower(value.Value))
	switch ProviderProtocol(s) {
	case ProtocolOpenAI, ProtocolClaude, ProtocolGemini:
		*p = ProviderProtocol(s)
		return nil
	case "":
		*p = ""
		return nil
	default:
		return fmt.Errorf("unsupported protocol %q", value.Value)
	}
}

func Load(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func ResolveProvider(cfg *Config, model string) (Provider, error) {
	if cfg == nil {
		return Provider{}, fmt.Errorf("config is nil")
	}
	return cfg.ResolveProvider(model)
}

func (c *Config) ResolveProvider(model string) (Provider, error) {
	if c == nil {
		return Provider{}, fmt.Errorf("config is nil")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return Provider{}, fmt.Errorf("model is required")
	}
	for _, route := range c.Routes {
		matched, err := path.Match(route.MatchModel, model)
		if err != nil {
			return Provider{}, fmt.Errorf("invalid route pattern %q: %w", route.MatchModel, err)
		}
		if !matched {
			continue
		}
		provider, ok := c.providerIndex[route.Provider]
		if !ok {
			return Provider{}, fmt.Errorf("provider %q not found", route.Provider)
		}
		return provider, nil
	}
	return Provider{}, fmt.Errorf("no provider matched model %q", model)
}

func (c *Config) validate() error {
	providerIndex := make(map[string]Provider, len(c.Providers))
	for i, provider := range c.Providers {
		provider.Name = strings.TrimSpace(provider.Name)
		provider.Endpoint = strings.TrimSpace(provider.Endpoint)
		if provider.Name == "" {
			return fmt.Errorf("providers[%d].name is required", i)
		}
		if provider.Protocol == "" {
			return fmt.Errorf("providers[%d].protocol is required", i)
		}
		if provider.Endpoint == "" {
			return fmt.Errorf("providers[%d].endpoint is required", i)
		}
		if _, exists := providerIndex[provider.Name]; exists {
			return fmt.Errorf("duplicate provider %q", provider.Name)
		}
		providerIndex[provider.Name] = provider
		c.Providers[i] = provider
	}

	for i, route := range c.Routes {
		route.MatchModel = strings.TrimSpace(route.MatchModel)
		route.Provider = strings.TrimSpace(route.Provider)
		if route.MatchModel == "" {
			return fmt.Errorf("routes[%d].match_model is required", i)
		}
		if route.Provider == "" {
			return fmt.Errorf("routes[%d].provider is required", i)
		}
		if _, ok := providerIndex[route.Provider]; !ok {
			return fmt.Errorf("routes[%d].provider %q does not exist", i, route.Provider)
		}
		c.Routes[i] = route
	}

	c.providerIndex = providerIndex
	return nil
}
