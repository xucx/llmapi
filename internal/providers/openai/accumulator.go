package openai

import (
	"fmt"
	"strings"

	"github.com/xucx/llmapi/internal/utils"

	"github.com/openai/openai-go/v2"
)

var (
	ThinkFlags = []string{"think"}
)

// extra think from message content or ExtraFields

type streamAccumulator struct {
	openai.ChatCompletionAccumulator
	thinks []thinkExtraItem
}

type thinkExtraItem struct {
	from  thinkFrom
	state int //0 wait, 1 thinking, 2 thinkover
	wait  string
	flag  string
	think string
}

type thinkFrom int

const (
	fromUnknown thinkFrom = iota
	fromExtra
	fromContent
)

func (t *thinkExtraItem) add(delta *openai.ChatCompletionChunkChoiceDelta) string {
	if t.from == fromUnknown {
		if _, ok := delta.JSON.ExtraFields["reasoning_content"]; ok {
			t.from = fromExtra
		} else {
			t.from = fromContent
		}
	}

	switch t.from {
	case fromExtra:
		if f, ok := delta.JSON.ExtraFields["reasoning_content"]; ok {
			thinkDelta := f.Raw()
			t.think += thinkDelta
			return thinkDelta
		}
	case fromContent:
		thinkDelta, contentDelta := t.extra(delta.Content)
		delta.Content = contentDelta
		return thinkDelta
	}
	return ""
}

func (t *thinkExtraItem) extra(text string) (string, string) {
	switch t.state {
	case 0:
		return t.checkWait(text)
	case 1:
		return t.checkThining(text)
	default:
		return "", text
	}
}

func (t *thinkExtraItem) checkWait(text string) (string, string) {
	if t.wait == "" {
		text = strings.TrimLeft(text, "\n")
	} else {
		text = t.wait + text
		t.wait = ""
	}

	for _, flag := range ThinkFlags {
		start := fmt.Sprintf("<%s>", flag)
		// matched
		if strings.HasPrefix(text, start) {
			t.flag = flag
			t.state = 1
			return t.checkThining(text[len(start):])
		}

		// keep wait
		if strings.HasPrefix(start, text) {
			t.wait = text
			return "", ""
		}
	}

	t.state = 2
	return "", text
}

func (t *thinkExtraItem) checkThining(text string) (string, string) {

	text = t.wait + text
	t.wait = ""

	end := fmt.Sprintf("</%s>", t.flag)
	if pos := strings.Index(text, end); pos > -1 {
		t.state = 2
		t.think += text[:pos]
		return text[:pos], text[pos+len(end):]
	}

	for i := 1; i < len(end); i++ {
		if strings.HasSuffix(text, end[:i]) {
			t.think += text[:len(text)-i]
			t.wait = text[len(text)-i:]
			return text[:len(text)-i], ""
		}
	}

	t.think += text
	return text, ""
}

func (s *streamAccumulator) Add(chunk *openai.ChatCompletionChunk) ([]string, bool) {
	thinks := make([]string, len(chunk.Choices))
	for i := range chunk.Choices {
		choise := &chunk.Choices[i]
		s.thinks = utils.ExpandSliceToFit(s.thinks, int(choise.Index))
		think := &s.thinks[int(choise.Index)]
		think.add(&choise.Delta)
	}
	return thinks, s.AddChunk(*chunk)
}

func extraReasongFromFullContent(content string) (string, string) {
	content = strings.TrimSpace(content)

	for _, flag := range ThinkFlags {
		start, end := fmt.Sprintf("<%s>", flag), fmt.Sprintf("</%s>", flag)
		startPos := strings.Index(content, start)
		endPos := strings.LastIndex(content, end)
		if startPos == 0 && endPos > -1 {
			return content[len(start):endPos], content[endPos+len(end):]
		}
	}

	return "", content
}
