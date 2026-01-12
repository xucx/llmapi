package llmapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	apiv1 "github.com/xucx/llmapi/api/v1"
	v1 "github.com/xucx/llmapi/api/v1"
	"github.com/xucx/llmapi/log"
	"github.com/xucx/llmapi/types"
)

func (p *LLMApiProvider) Generate(ctx context.Context, messages []*types.Message, options ...types.ChatOption) (*types.Completion, error) {
	opts := types.GetChatOptions(&types.ChatOptions{}, options...)

	chatParams, err := ChatOptionsToParams(messages, opts)
	if err != nil {
		return nil, err
	}

	if opts.StreamingFunc == nil && opts.StreamingAccFunc == nil {
		rsp, err := p.apiClient.Chat(ctx, &v1.ChatRequest{ChatParams: chatParams})
		if err != nil {
			return nil, err
		}
		return ToChatCompletion(rsp.ChatCompletion)
	} else {
		stream, err := p.apiClient.ChatStream(ctx, &v1.ChatStreamRequest{ChatParams: chatParams})
		if err != nil {
			return nil, err
		}

		acc := accChatCompletion{
			Completion: &v1.ChatCompletion{},
		}
		for {
			rsp, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil, errors.New("not finished")
				}
				return nil, err
			}

			if rsp.ChatCompletion.Delta {

				acc.add(rsp.ChatCompletion)

				if opts.StreamingFunc != nil {
					streamComplate, err := ToChatCompletion(rsp.ChatCompletion)
					if err != nil {
						return nil, err
					}
					if err := opts.StreamingFunc(ctx, streamComplate); err != nil {
						return nil, err
					}
				}

				if opts.StreamingAccFunc != nil {
					accComplate, err := ToChatCompletion(acc.Completion)
					if err != nil {
						return nil, err
					}
					log.Debugw("proxy return stream delta acc completion")
					if err := opts.StreamingAccFunc(ctx, accComplate); err != nil {
						return nil, err
					}
				}

				continue
			}

			return ToChatCompletion(rsp.ChatCompletion)

		}
	}
}

// Notice: we only merge reasoning and text now
type accChatCompletion struct {
	Completion *v1.ChatCompletion
	reasoning  string
	text       string
}

func (acc *accChatCompletion) add(c *v1.ChatCompletion) {
	if !c.Delta {
		return
	}

	acc.Completion.Delta = true
	acc.Completion.Model = c.Model
	acc.Completion.Message = &v1.ChatMessage{
		Role: "assistant",
	}

	if c.Message != nil {

		for _, c := range c.Message.Contents {
			if reasoningContent := c.GetReasoning(); reasoningContent != nil {
				acc.reasoning += reasoningContent.Text
			}

			if textContent := c.GetText(); textContent != nil {
				acc.text += textContent.Text
			}
		}

		acc.Completion.Message.Id = c.Message.Id
		acc.Completion.Message.Contents = []*v1.ChatContent{}
		if acc.reasoning != "" {
			acc.Completion.Message.Contents = append(acc.Completion.Message.Contents, &v1.ChatContent{Content: &v1.ChatContent_Reasoning{Reasoning: &v1.ChatContentReasoning{
				Text: acc.reasoning,
			}}})
		}

		if acc.text != "" {
			acc.Completion.Message.Contents = append(acc.Completion.Message.Contents, &v1.ChatContent{Content: &v1.ChatContent_Text{Text: &v1.ChatContentText{
				Text: acc.text,
			}}})
		}
	}
}

func ToChatCompletion(from *apiv1.ChatCompletion) (*types.Completion, error) {
	message, err := ToMessage(from.Message)
	if err != nil {
		return nil, err
	}

	usage := types.CompletionUsage{}
	if from.Usage != nil {
		usage.PromptTokens = from.Usage.PromptTokens
		usage.CompletionTokens = from.Usage.CompletionTokens
		usage.TotalTokens = from.Usage.TotalTokens
	}

	return &types.Completion{
		Delta:   from.Delta,
		Model:   from.Model,
		Message: message,
		Usage:   usage,
	}, nil
}

func FromChatCompletion(completion *types.Completion) (*apiv1.ChatCompletion, error) {
	if completion.Message == nil {
		return nil, errors.New("no completion message")
	}

	message, err := FromMessage(completion.Message)
	if err != nil {
		return nil, err
	}

	return &apiv1.ChatCompletion{
		Model:   completion.Model,
		Delta:   completion.Delta,
		Message: message,
		Usage: &apiv1.ChageUsage{
			PromptTokens:     completion.Usage.PromptTokens,
			CompletionTokens: completion.Usage.CompletionTokens,
			TotalTokens:      completion.Usage.TotalTokens,
		},
	}, nil
}

func ToMessage(from *apiv1.ChatMessage) (*types.Message, error) {
	to := &types.Message{
		ID: from.Id,
	}

	switch from.Role {
	case "system":
		to.Role = types.MessageRoleSystem
	case "assistant":
		to.Role = types.MessageRoleAssistant
	case "user":
		to.Role = types.MessageRoleUser
	case "tool":
		to.Role = types.MessageRoleTool
	default:
		return nil, fmt.Errorf("unknown role %s", from.Role)
	}

	for _, p := range from.Contents {
		if text := p.GetText(); text != nil {
			to.Parts = append(to.Parts, &types.MessageText{
				Text: text.Text,
			})
		} else if reasoning := p.GetReasoning(); reasoning != nil {
			to.Parts = append(to.Parts, &types.MessageReasoning{
				Text:             reasoning.Text,
				ThoughtSignature: reasoning.ThoughtSignature,
			})
		} else if toolCall := p.GetToolCall(); toolCall != nil {
			to.Parts = append(to.Parts, &types.MessageToolCall{
				ID:   toolCall.Id,
				Type: "function",
				Function: &types.ToolCallFunction{
					Name:      toolCall.Name,
					Arguments: toolCall.Arguments,
				},
			})
		} else if toolResult := p.GetToolResult(); toolResult != nil {
			to.Parts = append(to.Parts, &types.MessageToolResult{
				ID:     toolResult.Id,
				Name:   toolResult.Name,
				Result: toolResult.Result,
			})
		} else if audio := p.GetAudio(); audio != nil {
			to.Parts = append(to.Parts, &types.MessageAudio{
				Delta:      audio.Delta,
				Data:       audio.Data,
				Format:     audio.Format,
				Transcript: audio.Transcript,
			})
		} else {
			//
		}
	}

	return to, nil
}

func FromMessage(from *types.Message) (*apiv1.ChatMessage, error) {
	to := &apiv1.ChatMessage{
		Id:   from.ID,
		Role: string(from.Role),
	}

	for _, part := range from.Parts {
		switch p := part.(type) {
		case *types.MessageText:
			to.Contents = append(to.Contents, &apiv1.ChatContent{Content: &apiv1.ChatContent_Text{
				Text: &apiv1.ChatContentText{
					Text: p.Text,
				},
			}})
		case *types.MessageReasoning:
			to.Contents = append(to.Contents, &apiv1.ChatContent{Content: &apiv1.ChatContent_Reasoning{
				Reasoning: &apiv1.ChatContentReasoning{
					Text:             p.Text,
					ThoughtSignature: p.ThoughtSignature,
				},
			}})
		case *types.MessageRefusal:
			to.Contents = append(to.Contents, &apiv1.ChatContent{Content: &apiv1.ChatContent_Refusal{
				Refusal: &apiv1.ChatContentRefusal{
					Text: p.Text,
				},
			}})
		case *types.MessageToolCall:
			to.Contents = append(to.Contents, &apiv1.ChatContent{Content: &apiv1.ChatContent_ToolCall{
				ToolCall: &apiv1.ChatContentToolCall{
					Id:        p.ID,
					Name:      p.Function.Name,
					Arguments: p.Function.Arguments,
				},
			}})
		case *types.MessageToolResult:
			to.Contents = append(to.Contents, &apiv1.ChatContent{Content: &apiv1.ChatContent_ToolResult{
				ToolResult: &apiv1.ChatContentToolResult{
					Id:     p.ID,
					Name:   p.Name,
					Result: p.Result,
				},
			}})
		default:
			//
		}
	}

	return to, nil
}

func ChatParamsToOptions(req *apiv1.ChatParams) (*types.ChatOptions, error) {
	tools, err := ToChatTools(req.Tools)
	if err != nil {
		return nil, err
	}

	options := &types.ChatOptions{
		Model:        req.Model,
		Instructions: req.Instructions,
		Tools:        tools,
		AudioVoice:   types.AudioVoiceType(req.Voice),
	}
	return options, nil
}

func ToChatTools(tools []*apiv1.ChatTool) ([]*types.Tool, error) {
	all := []*types.Tool{}
	for _, t := range tools {
		params := map[string]any{}
		if err := json.Unmarshal([]byte(t.Params), &params); err != nil {
			return nil, err
		}

		all = append(all, &types.Tool{
			Type: types.ToolTypeFunction,
			Function: &types.ToolFunction{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  params,
			},
		})
	}

	return all, nil
}

func ToChatVoice(voice string) (types.AudioVoiceType, error) {
	switch voice {
	case "women":
		return types.AudioVoiceWomen, nil
	case "men":
		return types.AudioVoiceMen, nil
	default:
		return "", fmt.Errorf("voide %s not support", voice)
	}
}

func ToTurnDetection(turnDetection string) (types.TurnDetectionType, error) {
	switch turnDetection {
	case "server":
		return types.TurnDetectionServer, nil
	case "semantic":
		return types.TurnDetectionSemantic, nil
	default:
		return "", fmt.Errorf("turnDetection %s not support", turnDetection)
	}
}

func ChatOptionsToParams(messages []*types.Message, opts *types.ChatOptions) (*apiv1.ChatParams, error) {
	chatParams := &apiv1.ChatParams{
		Model:    opts.Model,
		Messages: []*apiv1.ChatMessage{},
		Tools:    []*apiv1.ChatTool{},
	}

	for _, msg := range messages {
		m, err := FromMessage(msg)
		if err != nil {
			return nil, err
		}
		chatParams.Messages = append(chatParams.Messages, m)
	}

	for _, tool := range opts.Tools {
		params, _ := json.Marshal(tool.Function.Parameters)
		chatParams.Tools = append(chatParams.Tools, &apiv1.ChatTool{
			Name:   tool.Function.Name,
			Desc:   tool.Function.Description,
			Params: string(params),
		})
	}

	return chatParams, nil
}
