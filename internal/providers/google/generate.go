package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xucx/llmapi/internal/utils"
	"github.com/xucx/llmapi/log"
	"github.com/xucx/llmapi/types"

	"github.com/google/uuid"
	"google.golang.org/genai"
)

func (p *GoogleProvider) Generate(ctx context.Context, messages []*types.Message, options ...types.ChatOption) (*types.Completion, error) {
	opts := types.GetChatOptions(&types.ChatOptions{
		Model: DefaultChatModel,
	}, options...)

	contents, contentsOpts, err := toChatContents(messages)
	if err != nil {
		return nil, err
	}

	config, err := toChatConfig(opts, contentsOpts)
	if err != nil {
		return nil, err
	}

	var (
		rsp *genai.GenerateContentResponse
	)

	if opts.StreamingFunc == nil && opts.StreamingAccFunc == nil {
		rsp, err = p.client.Models.GenerateContent(ctx, opts.Model, contents, config)
		if err != nil {
			return nil, err
		}
	} else {
		rsp = &genai.GenerateContentResponse{}

		stream := p.client.Models.GenerateContentStream(ctx, opts.Model, contents, config)
		for chunck, err := range stream {
			if err != nil {
				return nil, err
			}

			mergeChatStreamCompletion(rsp, chunck)

			if opts.StreamingFunc != nil {
				chunckComplete, err := fromChatCompletion(chunck, true)
				if err != nil {
					return nil, err
				}
				if err := opts.StreamingFunc(ctx, chunckComplete); err != nil {
					return nil, err
				}
			}

			if opts.StreamingAccFunc != nil {
				accComplete, err := fromChatCompletion(rsp, true)
				if err != nil {
					return nil, err
				}
				if err := opts.StreamingAccFunc(ctx, accComplete); err != nil {
					return nil, err
				}
			}
		}
	}

	return fromChatCompletion(rsp, false)
}

func toChatConfig(opts *types.ChatOptions, contentOpts *contentsOpt) (*genai.GenerateContentConfig, error) {
	config := &genai.GenerateContentConfig{
		ResponseModalities: []string{"TEXT"},
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  utils.Ptr[int32](8192),
		},
	}

	if opts.Instructions != "" {
		config.SystemInstruction = &genai.Content{
			Role: RoleSystem,
			Parts: []*genai.Part{
				{Text: opts.Instructions},
			},
		}
	}

	if opts.Temperature != nil {
		config.Temperature = opts.Temperature
	}

	for i, tool := range opts.Tools {
		if tool.Type != "function" {
			return nil, fmt.Errorf("tool [%d]: unsupported type %q, want 'function'", i, tool.Type)
		}

		config.Tools = append(config.Tools, &genai.Tool{FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:                 tool.Function.Name,
				Description:          tool.Function.Description,
				ParametersJsonSchema: tool.Function.Parameters,
			},
		}})
	}

	if contentOpts.hasAudio {
		config.ResponseModalities = append(config.ResponseModalities, "AUDIO")
		config.SpeechConfig = &genai.SpeechConfig{}
		if opts.AudioVoice != "" {
			voice, err := ToVoice(opts.AudioVoice)
			if err != nil {
				return nil, err
			}
			config.SpeechConfig.VoiceConfig = &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
					VoiceName: voice,
				},
			}
		}

	}

	return config, nil
}

type contentsOpt struct {
	hasAudio bool
}

func toChatContents(messages []*types.Message) ([]*genai.Content, *contentsOpt, error) {

	contents := []*genai.Content{}
	opts := &contentsOpt{}

	for _, msg := range messages {
		content := &genai.Content{}

		switch msg.Role {
		case types.MessageRoleSystem:
			content.Role = RoleSystem
		case types.MessageRoleAssistant:
			content.Role = RoleModel
		case types.MessageRoleUser:
			content.Role = RoleUser
		case types.MessageRoleTool:
			content.Role = RoleTool
		default:
			return nil, nil, fmt.Errorf("role %v not supported", msg.Role)
		}

		var thoughtSignature []byte

		for _, part := range msg.Parts {
			var toPart *genai.Part
			switch p := part.(type) {
			case *types.MessageReasoning:
				if p.ThoughtSignature != "" {
					thoughtSignature, _ = base64.StdEncoding.DecodeString(p.ThoughtSignature)
				}
			case *types.MessageText:
				toPart = &genai.Part{
					Text:             p.Text,
					ThoughtSignature: thoughtSignature,
				}
			case *types.MessageRefusal:
				// not support
			case *types.MessageImageURL:
				toPart = &genai.Part{
					FileData: &genai.FileData{
						FileURI: p.URL,
					},
				}
			case *types.MessageFile:
				toPart = &genai.Part{
					InlineData: &genai.Blob{
						MIMEType:    p.MIMEType,
						Data:        p.Data,
						DisplayName: p.Name,
					},
				}
			case *types.MessageAudio:
				if !p.Delta {
					data, err := base64.StdEncoding.DecodeString(p.Data)
					if err != nil {
						log.Fatal(err)
					}
					toPart = &genai.Part{
						InlineData: &genai.Blob{
							MIMEType: fmt.Sprintf("audio/%s", p.Format),
							Data:     data,
						},
					}
				}

			case *types.MessageToolCall:
				// only support fucntion tool call
				if p.Type != types.ToolTypeFunction || p.Function == nil {
					continue
				}

				args := map[string]any{}
				if err := json.Unmarshal([]byte(p.Function.Arguments), &args); err != nil {
					return nil, nil, err
				}

				toolCallthoughtSignature := thoughtSignature
				if len(toolCallthoughtSignature) == 0 {
					toolCallthoughtSignature = []byte("context_engineering_is_the_way_to_go")
				}

				toPart = &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID:   p.ID,
						Name: p.Function.Name,
						Args: args,
					},
					ThoughtSignature: toolCallthoughtSignature,
				}
			case *types.MessageToolResult:
				toPart = &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						ID:       p.ID,
						Name:     p.Name,
						Response: map[string]any{"output": p.Result},
					},
				}
			}

			if toPart != nil {
				content.Parts = append(content.Parts, toPart)
			}
		}

		if len(content.Parts) > 0 {
			contents = append(contents, content)
		}
	}

	return contents, opts, nil
}

func mergeChatStreamCompletion(to *genai.GenerateContentResponse, delta *genai.GenerateContentResponse) bool {

	if len(to.ResponseID) == 0 {
		to.ResponseID = delta.ResponseID
	} else if to.ResponseID != delta.ResponseID {
		return false
	}

	to.SDKHTTPResponse = delta.SDKHTTPResponse
	to.CreateTime = delta.CreateTime
	to.ModelVersion = delta.ModelVersion
	to.PromptFeedback = delta.PromptFeedback
	to.ResponseID = delta.ResponseID
	to.UsageMetadata = delta.UsageMetadata

	for _, c := range delta.Candidates {
		//expand
		to.Candidates = expandSliceToFit(to.Candidates, int(c.Index))
		candidate := to.Candidates[c.Index]
		if candidate == nil {
			candidate = &genai.Candidate{}
			to.Candidates[c.Index] = candidate
		}

		candidate.CitationMetadata = c.CitationMetadata
		candidate.FinishMessage = c.FinishMessage
		candidate.TokenCount = c.TokenCount
		candidate.FinishReason = c.FinishReason
		candidate.URLContextMetadata = c.URLContextMetadata
		candidate.AvgLogprobs = c.AvgLogprobs
		candidate.GroundingMetadata = c.GroundingMetadata
		candidate.Index = c.Index
		candidate.LogprobsResult = c.LogprobsResult
		candidate.SafetyRatings = c.SafetyRatings

		if c.Content != nil {
			if candidate.Content == nil {
				candidate.Content = &genai.Content{}
			}

			candidate.Content.Role = c.Content.Role

			parts := candidate.Content.Parts
			for _, p := range c.Content.Parts {

				//merge text
				if p.Text != "" || len(p.ThoughtSignature) != 0 {
					mergerd := false
					for _, v := range parts {
						if v.Thought == p.Thought {
							v.Text += p.Text
							v.ThoughtSignature = p.ThoughtSignature
							mergerd = true
							break
						}
					}

					if !mergerd {
						parts = append(parts, &genai.Part{
							Thought:          p.Thought,
							ThoughtSignature: p.ThoughtSignature,
							Text:             p.Text,
						})
					}
				}

				if p.FunctionCall != nil || p.InlineData != nil || p.FileData != nil {
					parts = append(parts, p)
				}
			}
			candidate.Content.Parts = parts
		}
	}

	return true
}

func expandSliceToFit[T any](slice []T, index int) []T {
	if index < len(slice) {
		return slice
	}
	if index < cap(slice) {
		return slice[:index+1]
	}
	newSlice := make([]T, index+1)
	copy(newSlice, slice)
	return newSlice
}

func fromChatCompletion(rsp *genai.GenerateContentResponse, delta bool) (*types.Completion, error) {
	if len(rsp.Candidates) < 1 {
		return nil, fmt.Errorf("completion no candidates")
	}

	candidate := rsp.Candidates[0]
	if candidate.Content == nil {
		return nil, fmt.Errorf("completion candidate has no content")
	}

	message := &types.Message{
		ID:   rsp.ResponseID,
		Role: types.MessageRoleAssistant,
	}

	for _, part := range candidate.Content.Parts {

		thoughtSignature := ""
		if len(part.ThoughtSignature) > 0 {
			thoughtSignature = base64.StdEncoding.EncodeToString(part.ThoughtSignature)
		}

		if part.Text != "" {
			if part.Thought {
				message.Parts = append(message.Parts, &types.MessageReasoning{
					Text:             part.Text,
					ThoughtSignature: thoughtSignature,
				})
			} else {
				message.Parts = append(message.Parts, &types.MessageText{Text: part.Text})
			}
		}

		if part.InlineData != nil {
			if strings.HasPrefix(part.InlineData.MIMEType, "audio/") {
				format := strings.TrimPrefix(part.InlineData.MIMEType, "audio/")
				data := base64.StdEncoding.EncodeToString(part.InlineData.Data)
				message.Parts = append(message.Parts, &types.MessageAudio{
					Format: format,
					Data:   data,
				})
			}
		}

		if !delta {
			if part.FunctionCall != nil {
				args, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					return nil, err
				}
				id := part.FunctionCall.ID
				if id == "" {
					id = uuid.NewString()
				}
				toolCal := &types.MessageToolCall{
					ID: id,
					Function: &types.ToolCallFunction{
						Name:      part.FunctionCall.Name,
						Arguments: string(args),
					},
				}
				message.Parts = append(message.Parts, toolCal)
			}
		}
	}

	completion := &types.Completion{
		Delta:   delta,
		Model:   rsp.ModelVersion,
		Message: message,
		Usage:   types.CompletionUsage{},
	}

	if rsp.UsageMetadata != nil {
		completion.Usage.CompletionTokens = int64(rsp.UsageMetadata.CandidatesTokenCount)
		completion.Usage.PromptTokens = int64(rsp.UsageMetadata.PromptTokenCount)
		completion.Usage.TotalTokens = int64(rsp.UsageMetadata.TotalTokenCount)
	}

	return completion, nil
}

func ToVoice(in types.AudioVoiceType) (string, error) {
	switch in {
	case types.AudioVoiceWomen:
		return "Zephyr", nil
	case types.AudioVoiceMen:
		return "Gacrux", nil
	default:
		return "", fmt.Errorf("voice %s not support", in)
	}
}
