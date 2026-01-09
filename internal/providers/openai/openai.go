package openai

import (
	"bytes"
	"strings"
	"sync"

	"github.com/xucx/llmapi/internal/providers/provider"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

const (
	ProviderName   = "openai"
	DefaultBaseUrl = "https://api.openai.com/v1"
)

var (
	_ provider.Provider = (*OpenaiProvider)(nil)
)

type OpenaiProvider struct {
	provider.ProviderNop
	options *provider.ProviderOptions
	client  openai.Client
	bufPool *sync.Pool
}

func NewOpenaiProvider(opts ...provider.ProviderOption) (provider.Provider, error) {
	options := provider.GetProviderOptions(opts...)
	openaiOpts := []option.RequestOption{
		option.WithAPIKey(options.Sk),
	}

	if options.Url == "" {
		options.Url = DefaultBaseUrl
	}

	url := options.Url
	if !strings.HasPrefix(url, "http") {
		url += "https://" + url
	}
	openaiOpts = append(openaiOpts, option.WithBaseURL(url))

	client := openai.NewClient(openaiOpts...)

	return &OpenaiProvider{
		options: options,
		client:  client,
		bufPool: &sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
	}, nil
}
