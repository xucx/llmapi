package llmapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xucx/llmapi/internal/providers"
	"github.com/xucx/llmapi/internal/providers/provider"
	"github.com/xucx/llmapi/types"
)

const (
	DefaultMaxToken = 32000
)

type Config struct {
	Providers []ProviderConfig `yaml:"providers"`
	Models    []ModelConfig    `yaml:"models"`
}

type ProviderConfig struct {
	Name     string `yaml:"name"`
	Provider string `yaml:"provider"`
	Url      string `yaml:"url"`
	Insecure bool   `yaml:"insecure"`
	Sk       string `yaml:"sk"`
}

type ModelConfig struct {
	Name     string `yaml:"name"`
	Model    string `yaml:"model"`
	Provider string `yaml:"provider"`
	MaxToken int64  `yaml:"maxToken"`
}

type Model struct {
	Name     string
	Model    string
	Provider provider.Provider
	MaxToken int64
}

type Models struct {
	providers map[string]provider.Provider
	models    map[string]*Model
}

func NewProvider(name string, opts ...provider.ProviderOption) (provider.Provider, error) {
	if creater, ok := providers.Creators[name]; ok {
		return creater(opts...)
	}

	return nil, fmt.Errorf("provider %s no support", name)
}

func NewProviderFromConfig(conf ProviderConfig) (provider.Provider, error) {
	if creater, ok := providers.Creators[conf.Provider]; ok {
		opts := []provider.ProviderOption{}
		if conf.Url != "" {
			opts = append(opts, provider.WithUrl(conf.Url))
		}
		opts = append(opts, provider.WithInsecure(conf.Insecure))
		if conf.Sk != "" {
			opts = append(opts, provider.WithSk(conf.Sk))
		}
		return creater(opts...)
	}

	return nil, fmt.Errorf("provider %s no support", conf.Provider)
}

func NewModel(name, model string, maxToken int64, provider provider.Provider) (*Model, error) {
	if name == "" {
		return nil, errors.New("model name can not be empty")
	}

	if model == "" {
		model = name
	}

	return &Model{
		Provider: provider,
		Name:     name,
		Model:    model,
		MaxToken: maxToken,
	}, nil
}

func NewModelFromConfig(conf ProviderConfig, name, model string, maxToken int64) (*Model, error) {
	provider, err := NewProviderFromConfig(conf)
	if err != nil {
		return nil, err
	}

	return NewModel(name, model, maxToken, provider)
}

func (m *Model) Generate(ctx context.Context, messages []*types.Message, options ...types.ChatOption) (*types.Completion, error) {
	optionsWithModel := append(options, types.ChatWithModel(m.Model))
	return m.Provider.Generate(ctx, messages, optionsWithModel...)
}

func (m *Model) Realtime(ctx context.Context, messages []*types.Message, options ...types.RealTimeOption) (types.RealTimeSession, error) {
	optionsWithModel := append(options, types.RealTimeWithModel(m.Model))
	return m.Provider.Realtime(ctx, messages, optionsWithModel...)
}

func NewModels(conf Config) (*Models, error) {
	providers := map[string]provider.Provider{}
	for _, p := range conf.Providers {
		provider, err := NewProviderFromConfig(p)
		if err != nil {
			return nil, err
		}
		providers[p.Name] = provider
	}

	models := map[string]*Model{}
	for _, model := range conf.Models {
		if p, ok := providers[model.Provider]; ok {
			m, err := NewModel(model.Name, model.Model, model.MaxToken, p)
			if err != nil {
				return nil, err
			}
			models[model.Name] = m
		} else {
			return nil, fmt.Errorf("init model %s fail, can not find provider %s", model.Name, model.Provider)
		}
	}

	return &Models{providers: providers, models: models}, nil
}

func (m *Models) GetModel(name string) (*Model, error) {

	if md, ok := m.models[name]; ok {
		return md, nil
	}

	items := strings.SplitN(name, "/", 2)
	if len(items) == 2 {
		if provider, ok := m.providers[items[0]]; ok {
			return &Model{
				Name:     items[1],
				Model:    items[1],
				Provider: provider,
			}, nil
		}
	}

	return nil, fmt.Errorf("model %s not found", name)
}

func (m *Models) Generate(ctx context.Context, modelName string, messages []*types.Message, options ...types.ChatOption) (*types.Completion, error) {

	md, err := m.GetModel(modelName)
	if err != nil {
		return nil, err
	}

	return md.Generate(ctx, messages, options...)
}

func (m *Models) Realtime(ctx context.Context, modelName string, messages []*types.Message, options ...types.RealTimeOption) (types.RealTimeSession, error) {
	md, err := m.GetModel(modelName)
	if err != nil {
		return nil, err
	}
	return md.Realtime(ctx, messages, options...)
}
