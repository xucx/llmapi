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
	ID    string
	Role  MessageRole
	Parts []MessagePart

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
		switch p := part.(type) {
		case *MessageText:
			sb.WriteString(fmt.Sprintf("%s\n", p.Text))
		case *MessageToolCall:
			sb.WriteString(fmt.Sprintf("[tool] %s %s\n", p.Function.Name, p.Function.Arguments))
		case *MessageToolResult:
			sb.WriteString(fmt.Sprintf("[toolResult] %s %s\n", p.Name, p.Result))
		case *MessageReasoning:
			sb.WriteString(fmt.Sprintf("[think] %s\n", p.Text))
		default:
			sb.WriteString(fmt.Sprintf("[%T]...\n", part))
		}
	}

	return sb.String()
}

func (c *Message) Text() string {
	sb := strings.Builder{}
	for _, part := range c.Parts {
		if text, ok := part.(*MessageText); ok {
			sb.WriteString(text.Text)
		}
	}
	return sb.String()
}

func (c *Message) ToolCalls() []*MessageToolCall {
	toolCalls := []*MessageToolCall{}
	for _, part := range c.Parts {
		switch p := part.(type) {
		case *MessageToolCall:
			toolCalls = append(toolCalls, p)
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
		switch part.(type) {
		case *MessageReasoning:
			hasReasonPart = true
		default:
			hasOtherPart = true
		}
	}

	return hasReasonPart && !hasOtherPart
}

type MessagePart interface {
	isMessagePart()
}

func NewTextMessage(role MessageRole, text string) *Message {
	msg := NewMessage(role)
	msg.Parts = append(msg.Parts, &MessageText{Text: text})
	return msg
}

type MessageText struct {
	Text  string
	Delta bool
}

func (*MessageText) isMessagePart() {}

type MessageReasoning struct {
	Text             string
	ThoughtSignature string //for gemini
}

func (*MessageReasoning) isMessagePart() {}

type MessageRefusal struct {
	Text string
}

func (*MessageRefusal) isMessagePart() {}

type MessageImageURL struct {
	URL string
	// Any of "auto", "low", "high".
	Detail string
	Format string
}

func (*MessageImageURL) isMessagePart() {}

type MessageAudio struct {
	ID         string
	Data       string // Base64 encoded audio data.
	Format     string // Openai: Any of "wav", "mp3", "pcm16".
	Transcript string
	Delta      bool
}

func (*MessageAudio) isMessagePart() {}

type MessageFile struct {
	MIMEType string
	Name     string
	Data     []byte
}

func (*MessageFile) isMessagePart() {}

type MessageToolCall struct {
	ID       string
	Type     ToolType
	Function *ToolCallFunction
	Result   string //option
	Tip      string //option
	Error    error  //option
}

type ToolCallFunction struct {
	Name      string
	Arguments string
}

func (*MessageToolCall) isMessagePart() {}

type MessageToolResult struct {
	ID     string
	Name   string
	Result string
}

func (*MessageToolResult) isMessagePart() {}

type MessageRealtimeResponse struct {
}

func (*MessageRealtimeResponse) isMessagePart() {}

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
