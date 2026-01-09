package google

import (
	"context"

	"github.com/xucx/llmapi/internal/providers/provider"

	"google.golang.org/genai"
)

const (
	ProviderName     = "google"
	DefaultChatModel = "gemini-2.5-flash"
)

const (
	RoleSystem = "system"
	RoleModel  = "model"
	RoleUser   = "user"
	RoleTool   = "tool"
)

var (
	_ provider.Provider = (*GoogleProvider)(nil)
)

type GoogleProvider struct {
	provider.ProviderNop
	client *genai.Client
}

// use gemini
func NewGoogleProvider(opts ...provider.ProviderOption) (provider.Provider, error) {
	options := provider.GetProviderOptions(opts...)

	config := &genai.ClientConfig{
		APIKey: options.Sk,
	}

	client, err := genai.NewClient(context.Background(), config)
	if err != nil {
		return nil, err
	}

	return &GoogleProvider{client: client}, nil
}
