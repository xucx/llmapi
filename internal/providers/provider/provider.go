package provider

import (
	"context"
	"fmt"

	"github.com/xucx/llmapi/types"
)

type ProviderOptions struct {
	Url      string
	Insecure bool
	Sk       string
}

type ProviderOption func(*ProviderOptions) *ProviderOptions
type ProviderCreator func(opts ...ProviderOption) (Provider, error)

type Provider interface {
	Generate(ctx context.Context, messages []*types.Message, options ...types.ChatOption) (*types.Completion, error)
	Realtime(ctx context.Context, messages []*types.Message, options ...types.RealTimeOption) (types.RealTimeSession, error)
}

func WithOptions(options *ProviderOptions) ProviderOption {
	return func(opts *ProviderOptions) *ProviderOptions {
		return options
	}
}

func WithUrl(url string) ProviderOption {
	return func(opts *ProviderOptions) *ProviderOptions {
		opts.Url = url
		return opts
	}
}

func WithInsecure(insecure bool) ProviderOption {
	return func(opts *ProviderOptions) *ProviderOptions {
		opts.Insecure = insecure
		return opts
	}
}

func WithSk(sk string) ProviderOption {
	return func(opts *ProviderOptions) *ProviderOptions {
		opts.Sk = sk
		return opts
	}
}

func GetProviderOptions(opts ...ProviderOption) *ProviderOptions {
	all := &ProviderOptions{}
	for _, opt := range opts {
		all = opt(all)
	}

	return all
}

var _ Provider = (*ProviderNop)(nil)

type ProviderNop struct{}

func (ProviderNop) Generate(ctx context.Context, messages []*types.Message, options ...types.ChatOption) (*types.Completion, error) {
	return nil, fmt.Errorf("generate not impl")
}

func (ProviderNop) Realtime(ctx context.Context, messages []*types.Message, options ...types.RealTimeOption) (types.RealTimeSession, error) {
	return nil, fmt.Errorf("generate not impl")
}
