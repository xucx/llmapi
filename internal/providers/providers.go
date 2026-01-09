package providers

import (
	"github.com/xucx/llmapi/internal/providers/anthropic"
	"github.com/xucx/llmapi/internal/providers/google"
	"github.com/xucx/llmapi/internal/providers/llmapi"
	"github.com/xucx/llmapi/internal/providers/openai"
	"github.com/xucx/llmapi/internal/providers/provider"
)

var (
	Creators = map[string]provider.ProviderCreator{
		openai.ProviderName:    openai.NewOpenaiProvider,
		google.ProviderName:    google.NewGoogleProvider,
		anthropic.ProviderName: anthropic.NewAnthropicProvider,
		llmapi.ProviderName:    llmapi.NewLLMApiProvider,
	}
)
