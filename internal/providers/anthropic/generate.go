package anthropic

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/xucx/llmapi/types"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

func (p *AnthropicProvider) Generate(ctx context.Context, messages []*types.Message, options ...types.ChatOption) (*types.Completion, error) {
	opts := types.GetChatOptions(&types.ChatOptions{
		Model: DefaultChatModel,
	}, options...)

	params, err := toChatParams(messages, opts)
	if err != nil {
		return nil, err
	}

	var (
		rsp *anthropic.Message
	)

	if opts.StreamingFunc == nil {
		rsp, err = p.client.Messages.New(ctx, *params)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("anthropic generate streaming not impl")
	}

	return fromChatCompletion(rsp)
}

func toChatParams(messages []*types.Message, opts *types.ChatOptions) (*anthropic.MessageNewParams, error) {
	params := &anthropic.MessageNewParams{}

	params.Model = anthropic.Model(opts.Model)
	if opts.Temperature != nil {
		params.Temperature = anthropic.Float(float64(*opts.Temperature))
	}

	if err := toChatMessages(params, messages); err != nil {
		return nil, err
	}

	if err := toChatTools(params, opts); err != nil {
		return nil, err
	}

	return params, nil
}

func toChatMessages(params *anthropic.MessageNewParams, messages []*types.Message) error {
	for _, msg := range messages {
		toMessage := anthropic.MessageParam{}

		switch msg.Role {
		case types.MessageRoleSystem:
			toMessage.Role = RoleSystem
		case types.MessageRoleAssistant:
			toMessage.Role = RoleAssistant
		case types.MessageRoleUser:
			toMessage.Role = RoleUser
		case types.MessageRoleTool:
			toMessage.Role = RoleUser
		default:
			return fmt.Errorf("role %v not supported", msg.Role)
		}

		for _, part := range msg.Parts {
			var toPart *anthropic.ContentBlockParamUnion
			switch p := part.(type) {
			case *types.MessageText:
				toPart = &anthropic.ContentBlockParamUnion{
					OfText: &anthropic.TextBlockParam{
						Text: p.Text,
					},
				}
			case *types.MessageRefusal:
				// not support
			case *types.MessageImageURL:
				toPart = &anthropic.ContentBlockParamUnion{
					OfImage: &anthropic.ImageBlockParam{
						Source: anthropic.ImageBlockParamSourceUnion{
							OfURL: &anthropic.URLImageSourceParam{
								URL: p.URL,
							},
						},
					},
				}
			case *types.MessageFile:
				mime := p.MIMEType
				if mime == "" {
					mime = http.DetectContentType(p.Data)
				}

				base64Data := fmt.Sprintf(
					"data:%s,base64,%s",
					mime,
					base64.StdEncoding.EncodeToString(p.Data),
				)

				toPart = &anthropic.ContentBlockParamUnion{
					OfDocument: &anthropic.DocumentBlockParam{
						Source: anthropic.DocumentBlockParamSourceUnion{
							OfBase64: &anthropic.Base64PDFSourceParam{
								Data:      base64Data,
								MediaType: constant.ApplicationPDF(mime),
							},
						},
					},
				}

			case *types.MessageAudio:
				// not support
				continue
			case *types.MessageToolCall:
				// only support fucntion tool call
				if p.Type != types.ToolTypeFunction || p.Function == nil {
					continue
				}
				toPart = &anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    p.ID,
						Name:  p.Function.Name,
						Input: p.Function.Arguments,
					},
				}

			case *types.MessageToolResult:
				toPart = &anthropic.ContentBlockParamUnion{
					OfToolResult: &anthropic.ToolResultBlockParam{
						ToolUseID: p.ID,
						Content: []anthropic.ToolResultBlockParamContentUnion{
							{
								OfText: &anthropic.TextBlockParam{
									Text: p.Result,
								},
							},
						},
					},
				}
			}

			toMessage.Content = append(toMessage.Content, *toPart)
		}

		if toMessage.Role == RoleSystem {
			for _, part := range toMessage.Content {
				if part.OfText != nil {
					params.System = append(params.System, *part.OfText)
				}
			}
		} else {
			params.Messages = append(params.Messages, toMessage)
		}
	}

	return nil
}

func toChatTools(params *anthropic.MessageNewParams, opts *types.ChatOptions) error {

	for _, tool := range opts.Tools {
		if tool.Function == nil {
			continue
		}

		inputSchema := anthropic.ToolInputSchemaParam{}
		if v, ok := tool.Function.Parameters["properties"]; ok {
			inputSchema.Properties = v
		}
		if v, ok := tool.Function.Parameters["type"]; ok {
			if vv, ok := v.(string); ok {
				inputSchema.Type = constant.Object(vv)
			}
		}
		if v, ok := tool.Function.Parameters["required"]; ok {
			if vv, ok := v.([]string); ok {
				inputSchema.Required = vv
			}
		}

		toTool := anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Function.Name,
				Description: anthropic.String(tool.Function.Description),
				InputSchema: inputSchema,
			},
		}

		params.Tools = append(params.Tools, toTool)
	}

	return nil
}

func fromChatCompletion(msg *anthropic.Message) (*types.Completion, error) {

	completion := &types.Completion{
		Model: string(msg.Model),
		Message: &types.Message{
			Role: types.MessageRoleAssistant,
		},
		Usage: types.CompletionUsage{
			CompletionTokens: msg.Usage.OutputTokens,
			PromptTokens:     msg.Usage.InputTokens,
			TotalTokens:      msg.Usage.InputTokens + msg.Usage.OutputTokens,
		},
	}

	var (
		contentBuf   = strings.Builder{}
		reasoningBuf = strings.Builder{}
	)

	for _, c := range msg.Content {
		switch variant := c.AsAny().(type) {
		case anthropic.ThinkingBlock:
			reasoningBuf.WriteString(variant.Thinking)
		case anthropic.TextBlock:
			contentBuf.WriteString(variant.Text)
		case anthropic.ToolUseBlock:
			completion.Message.Parts = append(completion.Message.Parts, &types.MessageToolCall{
				ID:   variant.ID,
				Type: "function",
				Function: &types.ToolCallFunction{
					Name:      variant.Name,
					Arguments: string(variant.Input),
				},
			})

		case anthropic.RedactedThinkingBlock:
			//
		}
	}

	reasoning := reasoningBuf.String()
	if reasoning != "" {
		completion.Message.Parts = append(completion.Message.Parts, &types.MessageReasoning{Text: reasoning})
	}
	content := contentBuf.String()
	if content != "" {
		completion.Message.Parts = append(completion.Message.Parts, &types.MessageText{Text: content})
	}

	return completion, nil
}
