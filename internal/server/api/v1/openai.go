package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/xucx/llmapi/log"
	"github.com/xucx/llmapi/types"
)

// see https://platform.openai.com/docs/api-reference/chat/create
type OpenaiCompletionRequest struct {
	Messages         []OpenaiMessage `json:"messages,omitempty"`
	Model            string          `json:"model,omitempty"`
	Tools            []OpenaiTool    `json:"tools,omitempty"`
	Stream           bool            `json:"stream,omitempty"`
	Temperature      *float32        `json:"temperature,omitempty"`
	MaxTokens        int64           `json:"max_tokens,omitempty"`
	PresencePenalty  float32         `json:"presence_penalty,omitempty"`
	FrequencyPenalty float32         `json:"frequency_penalty,omitempty"`
}

type OpenaiMessage struct {
	Role             string           `json:"role,omitempty"`
	Content          any              `json:"content,omitempty"`
	ReasoningContent any              `json:"reasoning_content,omitempty"`
	Name             *string          `json:"name,omitempty"`
	ToolCallId       string           `json:"tool_call_id,omitempty"`
	ToolCalls        []OpenaiToolCall `json:"tool_calls,omitempty"`
}

type OpenaiTool struct {
	Type     string         `json:"type,omitempty"`
	Function OpenaiFunction `json:"function"`
}

type OpenaiFunction struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Arguments   string `json:"arguments,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type OpenaiCompletionResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenaiChoice `json:"choices"`
	Usage   *OpenaiUsage   `json:"usage,omitempty"`
}

type OpenaiToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function OpenaiFunction `json:"function"`
}

type OpenaiChoice struct {
	Index        int               `json:"index"`
	Message      *OpenaiCompletion `json:"message,omitempty"`
	Delta        *OpenaiCompletion `json:"delta,omitempty"`
	FinishReason *string           `json:"finish_reason"`
}

type OpenaiCompletion struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content"` // Response content is string (or null)
	ToolCalls []OpenaiToolCall `json:"tool_calls,omitempty"`
	Refusal   *string          `json:"refusal,omitempty"`
}

type OpenaiUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

func (s *ApiService) OpenaiCompletion(c echo.Context) error {
	req := &OpenaiCompletionRequest{}
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ctx := c.Request().Context()

	// Convert messages
	messages := []*types.Message{}
	for _, m := range req.Messages {
		msg, err := fromOpenaiMessage(m)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		messages = append(messages, msg)
	}

	// Convert options
	options := []types.ChatOption{}
	if req.Model != "" {
		options = append(options, types.ChatWithModel(req.Model))
	}

	if len(req.Tools) > 0 {
		tools, err := fromOpenaiTools(req.Tools)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		options = append(options, types.ChatWithTools(tools))
	}

	// Temperature
	if req.Temperature != nil {
		options = append(options, func(opts *types.ChatOptions) *types.ChatOptions {
			opts.Temperature = req.Temperature
			return opts
		})
	}

	// Generate
	if req.Stream {
		c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
		c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
		c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
		c.Response().WriteHeader(http.StatusOK)

		options = append(options, types.ChatWithStreamingFunc(func(ctx context.Context, completion *types.Completion) error {
			resp, err := toOpenaiCompletionResponse(completion)
			if err != nil {
				return err
			}

			chunkData, err := json.Marshal(resp)
			if err != nil {
				return err
			}

			fmt.Fprintf(c.Response(), "data: %s\n\n", chunkData)
			c.Response().Flush()
			return nil
		}))

		_, err := s.models.Generate(ctx, req.Model, messages, options...)
		if err != nil {
			log.Errorw("llm chat fail", "model", req.Model, "error", err)
			return nil
		}

		fmt.Fprintf(c.Response(), "data: [DONE]\n\n")
		c.Response().Flush()
		return nil

	} else {
		completion, err := s.models.Generate(ctx, req.Model, messages, options...)
		if err != nil {
			log.Errorw("llm chat fail", "model", req.Model, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		resp, err := toOpenaiCompletionResponse(completion)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, resp)
	}
}

func fromOpenaiMessage(m OpenaiMessage) (*types.Message, error) {
	role := types.MessageRoleUser
	switch m.Role {
	case "system":
		role = types.MessageRoleSystem
	case "assistant":
		role = types.MessageRoleAssistant
	case "user":
		role = types.MessageRoleUser
	case "tool":
		role = types.MessageRoleTool
	default:
		if m.Role != "" {
			role = types.MessageRole(m.Role)
		}
	}

	msg := types.NewMessage(role)

	// Handle Content
	if m.Content != nil {
		switch c := m.Content.(type) {
		case string:
			msg.Parts = append(msg.Parts, &types.MessagePart{Text: &types.MessageText{Text: c}})
		case []interface{}:
			for _, item := range c {
				if itemMap, ok := item.(map[string]interface{}); ok {
					itemType, _ := itemMap["type"].(string)
					switch itemType {
					case "text":
						if text, ok := itemMap["text"].(string); ok {
							msg.Parts = append(msg.Parts, &types.MessagePart{Text: &types.MessageText{Text: text}})
						}
					case "image_url":
						if imageUrlObj, ok := itemMap["image_url"].(map[string]interface{}); ok {
							url, _ := imageUrlObj["url"].(string)
							msg.Parts = append(msg.Parts, &types.MessagePart{ImageURL: &types.MessageImageURL{URL: url}})
						}
					}
				}
			}
		}
	}

	// Handle Tool Calls
	if len(m.ToolCalls) > 0 {
		for _, tc := range m.ToolCalls {
			msg.Parts = append(msg.Parts, &types.MessagePart{ToolCall: &types.MessageToolCall{
				ID:   tc.ID,
				Type: types.ToolTypeFunction,
				Function: &types.ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}})
		}
	}

	// Handle Tool Result
	if role == types.MessageRoleTool {
		result := ""
		if s, ok := m.Content.(string); ok {
			result = s
		}
		msg.Parts = append(msg.Parts, &types.MessagePart{ToolResult: &types.MessageToolResult{
			ID:     m.ToolCallId,
			Result: result,
		}})
	}

	return msg, nil
}

func fromOpenaiTools(tools []OpenaiTool) ([]*types.Tool, error) {
	ts := []*types.Tool{}
	for _, t := range tools {
		if t.Type == "function" {
			paramsMap := map[string]any{}
			if params, ok := t.Function.Parameters.(map[string]any); ok {
				paramsMap = params
			} else if paramsStr, ok := t.Function.Parameters.(string); ok {
				json.Unmarshal([]byte(paramsStr), &paramsMap)
			}

			ts = append(ts, &types.Tool{
				Type: types.ToolTypeFunction,
				Function: &types.ToolFunction{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  paramsMap,
				},
			})
		}
	}
	return ts, nil
}

func toOpenaiCompletionResponse(c *types.Completion) (*OpenaiCompletionResponse, error) {
	resp := &OpenaiCompletionResponse{
		ID:      c.Message.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   c.Model,
		Choices: []OpenaiChoice{},
	}

	if c.Delta {
		resp.Object = "chat.completion.chunk"
	}

	choice := OpenaiChoice{
		Index: 0,
	}

	// Use OpenaiCompletion for response message
	msg := &OpenaiCompletion{
		Role: string(c.Message.Role),
	}

	content := c.Message.Text()
	if content != "" {
		msg.Content = content
	}

	toolCalls := c.Message.ToolCalls()
	if len(toolCalls) > 0 {
		openaiToolCalls := []OpenaiToolCall{}
		for _, tc := range toolCalls {
			openaiToolCalls = append(openaiToolCalls, OpenaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: OpenaiFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		msg.ToolCalls = openaiToolCalls
	}

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	if c.Delta {
		choice.Delta = msg
		choice.FinishReason = nil
	} else {
		choice.Message = msg
		choice.FinishReason = &finishReason
	}

	resp.Choices = append(resp.Choices, choice)

	if c.Usage.TotalTokens > 0 {
		resp.Usage = &OpenaiUsage{
			PromptTokens:     c.Usage.PromptTokens,
			CompletionTokens: c.Usage.CompletionTokens,
			TotalTokens:      c.Usage.TotalTokens,
		}
	}

	return resp, nil
}
