# llmapi

**llmapi** is a Go SDK for unified Large Language Model access, which can also be deployed as API Gateway.

## Supported Providers

- OpenAI
- Anthropic (Claude)
- Google (Gemini)

## Usage

```go
import "github.com/xucx/llmapi"

// 1. Configure
conf := llmapi.Config{
    Providers: []llmapi.ProviderConfig{
        {Name: "openai", Provider: "openai", Sk: "sk-..."},
        // other providers
    },
    Models: []llmapi.ModelConfig{
        {Name: "gpt-4", Provider: "openai", Model: "gpt-4"},
        // other models
    },
}

// 2. Initialize
models, _ := llmapi.NewModels(conf)

// 3. Use unified interface
resp, _ := models.Generate(ctx, "gpt-4", messages)
```

## Run as API Gateway

**Build & Run:**
```bash
make build
./dist/llmapi -c config.yaml
```

The server exposes:
- `POST /api/v1/openai/completions`
- `POST /api/v1/claude/messages`
- gRPC Service defined in `api/v1/`




