// AI 路由服务配置：解析 YAML、校验 Provider 和路由规则
package config

import (
	"fmt"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProviderProtocol 标识 AI Provider 使用的 API 协议
type ProviderProtocol string

const (
	ProtocolUnknown ProviderProtocol = "unknown"
	ProtocolOpenAI  ProviderProtocol = "openai"
	ProtocolClaude  ProviderProtocol = "claude"
	ProtocolGemini  ProviderProtocol = "gemini"
)

// Provider 描述一个 AI 服务提供商的连接参数
type Provider struct {
	Name     string           `yaml:"name" json:"name"`
	Protocol ProviderProtocol `yaml:"protocol" json:"protocol"`
	Endpoint string           `yaml:"endpoint" json:"endpoint"`
	APIKey   string           `yaml:"api_key" json:"-"`
	Models   []string         `yaml:"models" json:"models,omitempty"`
}

// Route 将模型名称（支持通配符）映射到 Provider
type Route struct {
	MatchModel string `yaml:"match_model" json:"match_model"`
	Provider   string `yaml:"provider" json:"provider"`
}

// Server HTTP 服务监听配置
type Server struct {
	Host     string   `yaml:"host" json:"host"`
	Port     int      `yaml:"port" json:"port"`
	LogLevel string   `yaml:"log_level" json:"log_level"`
	APIKeys  []string `yaml:"api_keys" json:"-"`
}

// Config 应用全局配置
type Config struct {
	Providers []Provider `yaml:"providers"`
	Routes    []Route    `yaml:"routes"`
	Server    Server     `yaml:"server"`

	providerIndex map[string]Provider `yaml:"-"`
}

// UnmarshalYAML 解析协议字段，不区分大小写，不支持的值返回错误
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

// Load 从 YAML 文件加载并校验配置
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

// ResolveProvider 包级别便捷函数，按模型名匹配 Provider
func ResolveProvider(cfg *Config, model string) (Provider, error) {
	if cfg == nil {
		return Provider{}, fmt.Errorf("config is nil")
	}
	return cfg.ResolveProvider(model)
}

// ResolveProvider 按路由规则匹配模型名，返回对应 Provider；使用 path.Match 通配符匹配
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

// validate 校验 Provider 和 Route 完整性，并构建 providerIndex 加速查找
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
