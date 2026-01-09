package anthropic

import (
	"github.com/xucx/llmapi/internal/providers/provider"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	ProviderName     = "anthropic"
	DefaultChatModel = string(anthropic.ModelClaudeSonnet4_0)
)

const (
	RoleSystem    = "system"
	RoleAssistant = "assistant"
	RoleUser      = "user"
)

var (
	_ provider.Provider = (*AnthropicProvider)(nil)
)

type AnthropicProvider struct {
	provider.ProviderNop
	client *anthropic.Client
}

func NewAnthropicProvider(opts ...provider.ProviderOption) (provider.Provider, error) {
	options := provider.GetProviderOptions(opts...)

	config := []option.RequestOption{
		option.WithAPIKey(options.Sk),
	}

	client := anthropic.NewClient(config...)
	return &AnthropicProvider{
		client: &client,
	}, nil
}
