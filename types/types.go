package types

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Tool struct {
	Type     ToolType
	Function *ToolFunction
}

type ToolType string

const (
	ToolTypeFunction ToolType = "function"
)

type ToolFunction struct {
	Name        string
	Description string
	Parameters  map[string]any
}

func NewFunctionTool(name, desc string, params map[string]any) *Tool {
	return &Tool{
		Type: ToolTypeFunction,
		Function: &ToolFunction{
			Name:        name,
			Description: desc,
			Parameters:  params,
		},
	}
}

type MessageRole string

const (
	MessageRoleSystem    MessageRole = "system"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleUser      MessageRole = "user"
	MessageRoleTool      MessageRole = "tool"
)

type Message struct {
	ID    string         `json:"id,omitempty" yaml:"id,omitempty"`
	Role  MessageRole    `json:"role,omitempty" yaml:"role,omitempty"`
	Parts []*MessagePart `json:"parts,omitempty" yaml:"parts,omitempty"`

	info map[string]any
}

func NewMessage(role MessageRole) *Message {
	return &Message{
		ID:   uuid.NewString(),
		Role: role,
	}
}

func (m *Message) SetInfo(key string, v any) {
	if m.info == nil {
		m.info = map[string]any{}
	}

	m.info[key] = v
}

func (m *Message) GetInfo(key string) (any, bool) {
	v, ok := m.info[key]
	return v, ok
}

func (m *Message) StringInfo(key string) (string, bool) {
	if v, ok := m.GetInfo(key); ok {
		if vv, ok := v.(string); ok {
			return vv, true
		}
	}
	return "", false
}

func (m *Message) String() string {
	sb := strings.Builder{}
	sb.WriteString(string(m.Role) + ":\n")
	for _, part := range m.Parts {
		switch {
		case part.Text != nil:
			sb.WriteString(fmt.Sprintf("%s\n", part.Text.Text))
		case part.ToolCall != nil:
			sb.WriteString(fmt.Sprintf("[tool] %s %s\n", part.ToolCall.Function.Name, part.ToolCall.Function.Arguments))
		case part.ToolResult != nil:
			sb.WriteString(fmt.Sprintf("[toolResult] %s %s\n", part.ToolResult.Name, part.ToolResult.Result))
		case part.Reasoning != nil:
			sb.WriteString(fmt.Sprintf("[think] %s\n", part.Reasoning.Text))
		default:
			//
		}
	}

	return sb.String()
}

func (c *Message) Text() string {
	sb := strings.Builder{}
	for _, part := range c.Parts {
		if part.Text != nil {
			sb.WriteString(part.Text.Text)
		}
	}
	return sb.String()
}

func (c *Message) ToolCalls() []*MessageToolCall {
	toolCalls := []*MessageToolCall{}
	for _, part := range c.Parts {
		if part.ToolCall != nil {
			toolCalls = append(toolCalls, part.ToolCall)
		}
	}
	return toolCalls
}

func (c *Message) IsReasoning() bool {
	if c.Role != MessageRoleAssistant {
		return false
	}

	hasReasonPart := false
	hasOtherPart := false
	for _, part := range c.Parts {
		if part.Reasoning != nil {
			hasReasonPart = true
		} else {
			hasOtherPart = false
		}
	}

	return hasReasonPart && !hasOtherPart
}

type MessagePart struct {
	// oneof
	Text             *MessageText             `json:"text,omitempty" yaml:"text,omitempty"`
	Reasoning        *MessageReasoning        `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
	Refusal          *MessageRefusal          `json:"refusal,omitempty" yaml:"refusal,omitempty"`
	ImageURL         *MessageImageURL         `json:"imageurl,omitempty" yaml:"imageurl,omitempty"`
	Audio            *MessageAudio            `json:"audio,omitempty" yaml:"audio,omitempty"`
	File             *MessageFile             `json:"file,omitempty" yaml:"file,omitempty"`
	ToolCall         *MessageToolCall         `json:"toolcall,omitempty" yaml:"toolcall,omitempty"`
	ToolResult       *MessageToolResult       `json:"toolresult,omitempty" yaml:"toolresult,omitempty"`
	RealtimeResponse *MessageRealtimeResponse `json:"realtimeresponse,omitempty" yaml:"realtimeresponse,omitempty"`
}

func NewTextMessage(role MessageRole, text string) *Message {
	msg := NewMessage(role)
	msg.Parts = append(msg.Parts, &MessagePart{Text: &MessageText{Text: text}})
	return msg
}

type MessageText struct {
	Text  string `json:"text,omitempty" yaml:"text,omitempty"`
	Delta bool   `json:"delta,omitempty" yaml:"delta,omitempty"`
}

type MessageReasoning struct {
	Text             string `json:"text,omitempty" yaml:"text,omitempty"`
	ThoughtSignature string `json:"thoughtSignature,omitempty" yaml:"thoughtSignature,omitempty"` //for gemini
}

type MessageRefusal struct {
	Text string `json:"text,omitempty" yaml:"text,omitempty"`
}

type MessageImageURL struct {
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
	// Any of "auto", "low", "high".
	Detail string `json:"detail,omitempty" yaml:"detail,omitempty"`
	Format string `json:"format,omitempty" yaml:"format,omitempty"`
}

type MessageAudio struct {
	ID         string `json:"id,omitempty" yaml:"id,omitempty"`
	Data       string `json:"data,omitempty" yaml:"data,omitempty"`     // base64 encoded audio data.
	Format     string `json:"format,omitempty" yaml:"format,omitempty"` // openai: any of "wav", "mp3", "pcm16".
	Transcript string `json:"transcript,omitempty" yaml:"transcript,omitempty"`
	Delta      bool   `json:"delta,omitempty" yaml:"delta,omitempty"`
}

type MessageFile struct {
	MIMEType string `json:"mimeType,omitempty" yaml:"mimeType,omitempty"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Data     string `json:"data,omitempty" yaml:"data,omitempty"` //base64
}

type MessageToolCall struct {
	ID       string            `json:"id,omitempty" yaml:"id,omitempty"`
	Type     ToolType          `json:"type,omitempty" yaml:"type,omitempty"`
	Function *ToolCallFunction `json:"function,omitempty" yaml:"function,omitempty"`
	Result   string            `json:"-" yaml:"-"` //option
	Tip      string            `json:"-" yaml:"-"` //option
	Error    error             `json:"-" yaml:"-"` //option
}

type ToolCallFunction struct {
	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	Arguments string `json:"arguments,omitempty" yaml:"arguments,omitempty"`
}

type MessageToolResult struct {
	ID     string `json:"id,omitempty" yaml:"id,omitempty"`
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
	Result string `json:"result,omitempty" yaml:"result,omitempty"`
}

type MessageRealtimeResponse struct {
}

type Completion struct {
	Delta   bool
	Model   string
	Message *Message
	Usage   CompletionUsage
}

type CompletionUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

type ChatOption func(*ChatOptions) *ChatOptions
type ChatOptions struct {
	Model            string
	Instructions     string
	Tools            []*Tool
	StreamingFunc    ChatStreamingFunc
	StreamingAccFunc ChatStreamingFunc
	Temperature      *float32
	TopP             *float32
	TopK             *int
	MaxTokens        *int64
	StopSequences    []string
	// Modalities    []Modality
	AudioVoice AudioVoiceType
}

func ChatWithOptions(options *ChatOptions) ChatOption {
	return func(opts *ChatOptions) *ChatOptions {
		return options
	}
}

func ChatWithModel(model string) ChatOption {
	return func(opts *ChatOptions) *ChatOptions {
		opts.Model = model
		return opts
	}
}

func ChatWithInstructions(Instructions string) ChatOption {
	return func(opts *ChatOptions) *ChatOptions {
		opts.Instructions = Instructions
		return opts
	}
}

func ChatWithAudioVoice(voice AudioVoiceType) ChatOption {
	return func(opts *ChatOptions) *ChatOptions {
		opts.AudioVoice = voice
		return opts
	}
}

func ChatWithTools(tools []*Tool) ChatOption {
	return func(opts *ChatOptions) *ChatOptions {
		opts.Tools = tools
		return opts
	}
}

func GetChatOptions(def *ChatOptions, opts ...ChatOption) *ChatOptions {
	if def == nil {
		def = &ChatOptions{}
	}
	for _, opt := range opts {
		def = opt(def)
	}
	return def
}

type ChatStreamingFunc func(ctx context.Context, completion *Completion) error

func ChatWithStreamingFunc(stream ChatStreamingFunc) ChatOption {
	return func(opts *ChatOptions) *ChatOptions {
		opts.StreamingFunc = stream
		return opts
	}
}

func ChatWithStreamingAccFunc(stream ChatStreamingFunc) ChatOption {
	return func(opts *ChatOptions) *ChatOptions {
		opts.StreamingAccFunc = stream
		return opts
	}
}

type AudioVoiceType string

var (
	AudioVoiceWomen AudioVoiceType = "women"
	AudioVoiceMen   AudioVoiceType = "men"
)

type Modality string

const (
	ModalityText  Modality = "text"
	ModalityAudio Modality = "audio"
)

type TurnDetectionType string

const (
	TurnDetectionServer   TurnDetectionType = "server"
	TurnDetectionSemantic TurnDetectionType = "semantic"
)

type RealTimeOptions struct {
	Model        string
	Instructions string
	Tools        []*Tool
	AudioVoice   *AudioVoiceType
}

type RealTimeOption func(*RealTimeOptions) *RealTimeOptions

func RealTimeWith(opts *RealTimeOptions) RealTimeOption {
	return func(opts *RealTimeOptions) *RealTimeOptions {
		return opts
	}
}

func RealTimeWithModel(model string) RealTimeOption {
	return func(opts *RealTimeOptions) *RealTimeOptions {
		opts.Model = model
		return opts
	}
}

func RealTimeWithInstructions(instructions string) RealTimeOption {
	return func(opts *RealTimeOptions) *RealTimeOptions {
		opts.Instructions = instructions
		return opts
	}
}

func RealTimeWithTools(tools []*Tool) RealTimeOption {
	return func(opts *RealTimeOptions) *RealTimeOptions {
		opts.Tools = tools
		return opts
	}
}

func RealTimeWithAudioVoice(voice AudioVoiceType) RealTimeOption {
	return func(opts *RealTimeOptions) *RealTimeOptions {
		opts.AudioVoice = &voice
		return opts
	}
}

type RealTimeSession interface {
	Send(context.Context, *Message) error
	Recv(context.Context) (*Completion, error)
	Close()
}
