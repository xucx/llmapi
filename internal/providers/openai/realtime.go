package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/xucx/llmapi/internal/httpx"
	"github.com/xucx/llmapi/types"

	"go.uber.org/multierr"
)

const (
	DefaultRealTimeModel = "gpt-4o-realtime-preview"
)

func (p *OpenaiProvider) Realtime(ctx context.Context, messages []*types.Message, options ...types.RealTimeOption) (types.RealTimeSession, error) {
	option := &types.RealTimeOptions{
		Model: DefaultChatModel,
	}
	for _, opt := range options {
		option = opt(option)
	}

	url, err := p.getRealTimeUrl(option)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+p.options.Sk)

	client, err := httpx.WSConnect(ctx, url, httpx.WSClientOptions{
		Header: header,
	})
	if err != nil {
		return nil, err
	}

	client.SetReadLimit(10000000) //10M

	//update session
	sessionUpdateEvent, err := CreateSessionUpdateEvent(option)
	if err != nil {
		return nil, err
	}
	err = client.SendJsonMessage(ctx, sessionUpdateEvent)
	if err != nil {
		return nil, err
	}

	session := ClientSession{client: client}

	//send messages
	for _, msg := range messages {
		if err := session.Send(ctx, msg); err != nil {
			return nil, err
		}
	}

	return &session, nil
}

func (p *OpenaiProvider) getRealTimeUrl(opts *types.RealTimeOptions) (string, error) {
	query := url.Values{}
	query.Set("model", opts.Model)
	url, err := url.Parse(p.options.Url + "?" + query.Encode())
	if err != nil {
		return "", err
	}
	url = url.JoinPath("/realtime")
	switch url.Scheme {
	case "http":
		url.Scheme = "ws"
	default:
		url.Scheme = "wss"
	}

	return url.String(), nil
}

type ClientSession struct {
	client *httpx.WSClient
}

func (r *ClientSession) Send(ctx context.Context, msg *types.Message) error {

	var (
		allErr       error
		messageEvent = NewClientEventConversationItemCreate()
		otherEvents  = []any{}
	)

	messageEvent.Item.Type = "message"

	switch msg.Role {
	case types.MessageRoleSystem:
		messageEvent.Item.Role = "system"
		for _, p := range msg.Parts {
			switch pp := p.(type) {
			case *types.MessageText:
				messageEvent.Item.Content = append(messageEvent.Item.Content, &RealTimeContent{
					Type: "input_text",
					Text: pp.Text,
				})
			default:
				return fmt.Errorf("not support message for realtime system role")
			}
		}
	case types.MessageRoleUser:
		messageEvent.Item.Role = "user"
		for _, p := range msg.Parts {
			switch pp := p.(type) {
			case *types.MessageText:
				messageEvent.Item.Content = append(messageEvent.Item.Content, &RealTimeContent{
					Type: "input_text",
					Text: pp.Text,
				})

			case *types.MessageAudio:
				if pp.Format != "pcm16" {
					return fmt.Errorf("realtime audio format %s not support", pp.Format)
				}

				if !pp.Delta {
					messageEvent.Item.Content = append(messageEvent.Item.Content, &RealTimeContent{
						Type:  "input_audio",
						Audio: pp.Data,
					})
				} else {
					audioDeltaEvent := NewClientEventInputAudioBufferAppend()
					audioDeltaEvent.Audio = pp.Data
					otherEvents = append(otherEvents, audioDeltaEvent)
				}
			case *types.MessageImageURL:
				if !strings.EqualFold(pp.Format, "PNG") || !strings.EqualFold(pp.Format, "JPEG") {
					return fmt.Errorf("realtime image url format %s not support", pp.Format)
				}
				messageEvent.Item.Content = append(messageEvent.Item.Content, &RealTimeContent{
					Type:     "input_image",
					ImageUrl: pp.URL,
				})
			case *types.MessageRealtimeResponse:
				createRspEvent := NewClientEventResponseCreate()
				otherEvents = append(otherEvents, createRspEvent)
			default:
				return fmt.Errorf("unsupport realtime user message")
			}
		}

	case types.MessageRoleAssistant:
		messageEvent.Item.Role = "assistant"
		for _, p := range msg.Parts {
			switch pp := p.(type) {
			case *types.MessageText:
				messageEvent.Item.Content = append(messageEvent.Item.Content, &RealTimeContent{
					Type: "output_text",
					Text: pp.Text,
				})
			case *types.MessageToolCall:
				toolCallEvent := NewClientEventConversationItemCreate()
				toolCallEvent.Item.Type = "function_call"
				toolCallEvent.Item.CallID = pp.ID
				toolCallEvent.Item.Name = pp.Function.Name
				toolCallEvent.Item.Arguments = pp.Function.Arguments
				otherEvents = append(otherEvents, toolCallEvent)
			default:
				return fmt.Errorf("unsupport realtime assistant message")
			}
		}
	case types.MessageRoleTool:
		for _, p := range msg.Parts {
			switch pp := p.(type) {
			case *types.MessageToolResult:
				toolOutputEvent := NewClientEventConversationItemCreate()
				toolOutputEvent.Item.Type = "function_call_output"
				toolOutputEvent.Item.CallID = pp.ID
				toolOutputEvent.Item.Output = pp.Result
				otherEvents = append(otherEvents, toolOutputEvent)
			default:
				return fmt.Errorf("unsupport realtime tool message")
			}
		}
	default:
		return fmt.Errorf("realtime unsupport role %s", msg.Role)
	}

	if len(messageEvent.Item.Content) > 0 {
		if err := r.client.SendJsonMessage(ctx, messageEvent); err != nil {
			allErr = multierr.Append(allErr, err)
		}
	}

	for _, event := range otherEvents {
		if err := r.client.SendJsonMessage(ctx, event); err != nil {
			allErr = multierr.Append(allErr, err)
		}
	}

	return allErr
}

func (r *ClientSession) Recv(ctx context.Context) (*types.Completion, error) {
	messageType, data, err := r.client.Recv(ctx)
	if err != nil {
		return nil, err
	}
	if messageType != httpx.WSMessageText {
		return nil, fmt.Errorf("expected text message, got %d", messageType)
	}

	_, event, err := UnmarshalServerEvent(data)
	if err != nil {
		return nil, err
	}

	completion := &types.Completion{
		Message: &types.Message{
			Role: types.MessageRoleAssistant,
		},
	}

	switch p := event.(type) {
	case *ServerEventResponseOutputAudioDelta:
		completion.Delta = true
		completion.Message.Parts = append(completion.Message.Parts, &types.MessageAudio{
			Data:   p.Delta,
			Format: "pcm16",
			Delta:  true,
		})
	case *ServerEventResponseDone:
		if p.Response.Status != "completed" {
			break
		}

		for _, output := range p.Response.Output {
			switch output.Type {
			case "message":
				for _, content := range output.Content {
					switch content.Type {
					case "output_text":
						completion.Message.Parts = append(completion.Message.Parts, &types.MessageText{
							Text: content.Text,
						})
					case "output_audio":
						completion.Message.Parts = append(completion.Message.Parts, &types.MessageAudio{
							Transcript: content.Transcript,
						})
					}
				}
			case "function_call":
				completion.Message.Parts = append(completion.Message.Parts, &types.MessageToolCall{
					ID:   output.CallID,
					Type: types.ToolTypeFunction,
					Function: &types.ToolCallFunction{
						Name:      output.Name,
						Arguments: output.Arguments,
					},
				})
			}
		}
	case *Event:
		//
	default:

	}

	return completion, nil
}

func (r *ClientSession) Close() {
	r.client.Close()
}

func CreateSessionUpdateEvent(opts *types.RealTimeOptions) (*ClientEventSessionUpdate, error) {
	event := NewClientEventSessionUpdate()
	event.Session.Type = "realtime"
	event.Session.Model = opts.Model
	event.Session.Instructions = opts.Instructions
	event.Session.OutputModalities = []Modality{ModalityAudio}
	event.Session.Audio = &ClientEventSessionUpdateSessionAudio{
		Input: &ClientEventSessionUpdateSessionAudioInput{
			TurnDetection: &ClientEventSessionUpdateSessionAudioInputTurnDetection{
				Type:              ClientTurnDetectionTypeSemanticVad,
				CreateResponse:    true,
				InterruptResponse: true,
			},
		},
		Output: &ClientEventSessionUpdateSessionAudioOutput{
			Voice: VoiceAlloy,
		},
	}

	if opts.AudioVoice != nil {
		voice, err := ToVoice(*opts.AudioVoice)
		if err != nil {
			return nil, err
		}
		event.Session.Audio.Output.Voice = voice
	}

	for _, tool := range opts.Tools {
		if tool.Type == types.ToolTypeFunction {
			event.Session.Tools = append(event.Session.Tools, &ClientEventSessionUpdateSessionToolFunctions{
				Type:        "function",
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			})
		}
	}

	return event, nil
}

func ToModality(in types.Modality) (Modality, error) {
	switch in {
	case types.ModalityText:
		return ModalityText, nil
	case types.ModalityAudio:
		return ModalityAudio, nil
	default:
		return "", fmt.Errorf("unsupport modality %s ", in)
	}
}

func ToVoice(in types.AudioVoiceType) (Voice, error) {
	switch in {
	case types.AudioVoiceWomen:
		return VoiceAlloy, nil
	case types.AudioVoiceMen:
		return VoiceAsh, nil
	default:
		return "", fmt.Errorf("voice %s not support", in)
	}
}
