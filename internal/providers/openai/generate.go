package openai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"github.com/xucx/llmapi/types"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/shared"
)

const (
	DefaultChatModel = openai.ChatModelGPT4_1
)

func (p *OpenaiProvider) Generate(ctx context.Context, messages []*types.Message, options ...types.ChatOption) (*types.Completion, error) {

	opts := types.GetChatOptions(&types.ChatOptions{
		Model: DefaultChatModel,
	}, options...)

	params, err := toParams(opts, messages)
	if err != nil {
		return nil, err
	}

	if opts.StreamingFunc == nil && opts.StreamingAccFunc == nil {
		completion, err := p.client.Chat.Completions.New(ctx, *params)
		if err != nil {
			return nil, err
		}
		return fromComplate(completion, nil, false)
	}

	acc := streamAccumulator{}
	openaiStream := p.client.Chat.Completions.NewStreaming(ctx, *params)
	for openaiStream.Next() {

		chunk := openaiStream.Current()
		thinkChunks, _ := acc.Add(&chunk)

		if opts.StreamingFunc != nil {
			chunckCompletion, err := fromComplateChunk(&chunk, thinkChunks)
			if err != nil {
				return nil, err
			}
			if err = opts.StreamingFunc(ctx, chunckCompletion); err != nil {
				return nil, err
			}
		}

		if opts.StreamingAccFunc != nil {
			accCompletion, err := fromComplate(&acc.ChatCompletion, acc.thinks, true)

			if err != nil {
				return nil, err
			}
			if err = opts.StreamingAccFunc(ctx, accCompletion); err != nil {
				return nil, err
			}
		}
	}

	if err = openaiStream.Err(); err != nil {
		return nil, err
	}

	return fromComplate(&acc.ChatCompletion, acc.thinks, false)
}

type messageOpts struct {
	hasAudio bool
}

func toMessages(opts *types.ChatOptions, messages []*types.Message) ([]openai.ChatCompletionMessageParamUnion, *messageOpts, error) {
	messageOpts := &messageOpts{}
	openaiMsgs := []openai.ChatCompletionMessageParamUnion{}
	if opts.Instructions != "" {
		openaiMsgs = append(openaiMsgs, openai.SystemMessage(opts.Instructions))
	}

	for _, msg := range messages {
		switch msg.Role {
		case types.MessageRoleSystem:
			for _, part := range msg.Parts {
				switch p := part.(type) {
				case *types.MessageText:
					openaiMsgs = append(openaiMsgs, openai.SystemMessage(p.Text))
				default:
					//not support
				}
			}
		case types.MessageRoleUser:
			parts := []openai.ChatCompletionContentPartUnionParam{}
			for _, part := range msg.Parts {
				switch p := part.(type) {
				case *types.MessageText:
					parts = append(parts, openai.ChatCompletionContentPartUnionParam{
						OfText: &openai.ChatCompletionContentPartTextParam{
							Text: p.Text,
						},
					})
				case *types.MessageImageURL:
					parts = append(parts, openai.ChatCompletionContentPartUnionParam{
						OfImageURL: &openai.ChatCompletionContentPartImageParam{
							ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
								URL:    p.URL,
								Detail: p.Detail,
							},
						},
					})
				case *types.MessageAudio:
					messageOpts.hasAudio = true
					parts = append(parts, openai.ChatCompletionContentPartUnionParam{
						OfInputAudio: &openai.ChatCompletionContentPartInputAudioParam{
							InputAudio: openai.ChatCompletionContentPartInputAudioInputAudioParam{
								Data:   p.Data,
								Format: p.Format,
							},
						},
					})
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

					parts = append(parts, openai.ChatCompletionContentPartUnionParam{
						OfFile: &openai.ChatCompletionContentPartFileParam{
							File: openai.ChatCompletionContentPartFileFileParam{
								FileData: param.Opt[string]{Value: base64Data},
								Filename: param.Opt[string]{Value: p.Name},
							},
						},
					})
				}
			}
			if len(parts) > 0 {
				openaiMsgs = append(openaiMsgs, openai.UserMessage(parts))
			}
		case types.MessageRoleAssistant:
			assistant := openai.ChatCompletionAssistantMessageParam{
				Role: "assistant",
			}
			hasContent := false
			for _, part := range msg.Parts {
				switch p := part.(type) {
				case *types.MessageText:
					assistant.Content.OfString = openai.String(p.Text)
					hasContent = true
				case *types.MessageRefusal:
					assistant.Refusal = openai.String(p.Text)
					hasContent = true
				case *types.MessageAudio:
					if !p.Delta && p.ID != "" {
						messageOpts.hasAudio = true
						openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessageParamUnion{OfAssistant: &openai.ChatCompletionAssistantMessageParam{
							Role: "assistant",
							Content: openai.ChatCompletionAssistantMessageParamContentUnion{
								OfString: openai.String(p.Transcript),
							},
							// Audio: openai.ChatCompletionAssistantMessageParamAudio{
							// 	ID: p.ID,
							// },
						}})
					}
				case *types.MessageToolCall:
					//only support function call
					assistant.ToolCalls = append(assistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							Type: "function",
							ID:   p.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      p.Function.Name,
								Arguments: p.Function.Arguments,
							},
						},
					})
					hasContent = true
				}
			}
			if hasContent {
				openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant})
			}
		case types.MessageRoleTool:
			for _, part := range msg.Parts {
				switch p := part.(type) {
				case *types.MessageToolResult:
					openaiMsgs = append(openaiMsgs, openai.ToolMessage(p.Result, p.ID))
				default:
					//not support
				}
			}
		default:
			return nil, nil, fmt.Errorf("openai not support role [%s]", msg.Role)
		}
	}

	return openaiMsgs, messageOpts, nil
}

func toParams(opts *types.ChatOptions, messages []*types.Message) (*openai.ChatCompletionNewParams, error) {
	openaiMessages, messageOpts, err := toMessages(opts, messages)
	if err != nil {
		return nil, err
	}

	isStream := opts.StreamingFunc != nil || opts.StreamingAccFunc != nil

	openaiPramas := &openai.ChatCompletionNewParams{
		Model:    opts.Model,
		Messages: openaiMessages,
	}

	if isStream {
		openaiPramas.StreamOptions.IncludeUsage = openai.Bool(true)
	}

	if opts.Temperature != nil {
		openaiPramas.Temperature = openai.Float(float64(*opts.Temperature))
	}

	for _, tool := range opts.Tools {
		if tool.Type != types.ToolTypeFunction || tool.Function == nil {
			return nil, errors.New("openai only support function tool for now")
		}

		openaiPramas.Tools = append(openaiPramas.Tools, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        tool.Function.Name,
			Description: openai.String(tool.Function.Description),
			Parameters:  tool.Function.Parameters,
		}))
	}

	if messageOpts.hasAudio {
		openaiPramas.Audio = openai.ChatCompletionAudioParam{
			Format: openai.ChatCompletionAudioParamFormatMP3,
			Voice:  openai.ChatCompletionAudioParamVoice(VoiceAlloy),
		}
		openaiPramas.Modalities = []string{"text", "audio"}
		if opts.AudioVoice != "" {
			voice, err := ToVoice(opts.AudioVoice)
			if err != nil {
				return nil, err
			}
			openaiPramas.Audio.Voice = openai.ChatCompletionAudioParamVoice(voice)
		}
	}

	return openaiPramas, nil
}

func fromComplateChunk(completion *openai.ChatCompletionChunk, thinkChunks []string) (*types.Completion, error) {
	if len(completion.Choices) < 1 {
		return nil, fmt.Errorf("completion no choices")
	}

	choice := completion.Choices[0]
	message := &types.Message{
		ID:   completion.ID,
		Role: types.MessageRoleAssistant,
	}

	if len(thinkChunks) > 0 && len(thinkChunks[0]) > 0 {
		message.Parts = append(message.Parts, &types.MessageReasoning{Text: thinkChunks[0]})
	}

	if choice.Delta.Refusal != "" {
		message.Parts = append(message.Parts, &types.MessageRefusal{Text: choice.Delta.Content})
	}

	if choice.Delta.Content != "" {
		message.Parts = append(message.Parts, &types.MessageText{
			Delta: true,
			Text:  choice.Delta.Content,
		})
	}

	for _, toolCall := range choice.Delta.ToolCalls {
		if toolCall.Type != "function" {
			continue
		}
		message.Parts = append(message.Parts, &types.MessageToolCall{
			ID:   toolCall.ID,
			Type: types.ToolTypeFunction,
			Function: &types.ToolCallFunction{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		})
	}

	return &types.Completion{
		Delta:   true,
		Model:   completion.Model,
		Message: message,
		Usage: types.CompletionUsage{
			CompletionTokens: completion.Usage.CompletionTokens,
			PromptTokens:     completion.Usage.PromptTokens,
			TotalTokens:      completion.Usage.TotalTokens,
		},
	}, nil
}

func fromComplate(completion *openai.ChatCompletion, thinks []thinkExtraItem, delta bool) (*types.Completion, error) {

	if len(completion.Choices) < 1 {
		return nil, fmt.Errorf("completion no choices")
	}

	choice := completion.Choices[0]
	message := &types.Message{
		ID:   completion.ID,
		Role: types.MessageRoleAssistant,
	}

	reasoning := ""
	if len(thinks) > 0 {
		reasoning = thinks[0].think
	}

	content := choice.Message.Content
	if len(thinks) == 0 && !delta {
		reasoning, content = extraReasongFromFullContent(content)
	}

	if reasoning != "" {
		message.Parts = append(message.Parts, &types.MessageReasoning{Text: reasoning})
	}

	if content != "" {
		message.Parts = append(message.Parts, &types.MessageText{Text: content})
	}

	if choice.Message.Refusal != "" {
		message.Parts = append(message.Parts, &types.MessageRefusal{Text: choice.Message.Content})
	}

	if choice.Message.Audio.ID != "" {
		message.Parts = append(message.Parts, &types.MessageAudio{
			ID:         choice.Message.Audio.ID,
			Format:     "mp3",
			Data:       choice.Message.Audio.Data,
			Transcript: choice.Message.Audio.Transcript,
		})
	}

	if !delta {
		for _, toolCall := range choice.Message.ToolCalls {
			if toolCall.Type != "function" {
				continue
			}
			message.Parts = append(message.Parts, &types.MessageToolCall{
				ID:   toolCall.ID,
				Type: types.ToolTypeFunction,
				Function: &types.ToolCallFunction{
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				},
			})
		}
	}

	return &types.Completion{
		Delta:   delta,
		Model:   completion.Model,
		Message: message,
		Usage: types.CompletionUsage{
			CompletionTokens: completion.Usage.CompletionTokens,
			PromptTokens:     completion.Usage.PromptTokens,
			TotalTokens:      completion.Usage.TotalTokens,
		},
	}, nil
}
