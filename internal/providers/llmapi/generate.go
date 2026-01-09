package llmapi

import (
	"context"
	"errors"
	"io"

	v1 "github.com/xucx/llmapi/api/v1"
	v1util "github.com/xucx/llmapi/internal/server/api/v1/util"
	"github.com/xucx/llmapi/log"
	"github.com/xucx/llmapi/types"
)

func (p *LLMApiProvider) Generate(ctx context.Context, messages []*types.Message, options ...types.ChatOption) (*types.Completion, error) {
	opts := types.GetChatOptions(&types.ChatOptions{}, options...)

	chatParams, err := v1util.ChatOptionsToParams(messages, opts)
	if err != nil {
		return nil, err
	}

	if opts.StreamingFunc == nil && opts.StreamingAccFunc == nil {
		rsp, err := p.apiClient.Chat(ctx, &v1.ChatRequest{ChatParams: chatParams})
		if err != nil {
			return nil, err
		}
		return v1util.ToChatCompletion(rsp.ChatCompletion)
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
					streamComplate, err := v1util.ToChatCompletion(rsp.ChatCompletion)
					if err != nil {
						return nil, err
					}
					if err := opts.StreamingFunc(ctx, streamComplate); err != nil {
						return nil, err
					}
				}

				if opts.StreamingAccFunc != nil {
					accComplate, err := v1util.ToChatCompletion(acc.Completion)
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

			return v1util.ToChatCompletion(rsp.ChatCompletion)

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
