package httpx

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type SSEEventResponse[T any] struct {
	response   *http.Response
	reader     *bufio.Reader
	isFinished bool
}

func NewSSEEventResponse[T any](rsp *http.Response) *SSEEventResponse[T] {
	return &SSEEventResponse[T]{
		response: rsp,
		reader:   bufio.NewReader(rsp.Body),
	}
}

func (s *SSEEventResponse[T]) Recv() (*T, error) {
	if s.isFinished {
		return nil, io.EOF
	}

	return s.process()
}

func (s *SSEEventResponse[T]) Close() error {
	return s.response.Body.Close()
}

func (s *SSEEventResponse[T]) process() (*T, error) {
	lines := [][]byte{}
	for {
		line, err := s.reader.ReadBytes('\n')
		trimedLine := bytes.TrimSpace(line)
		if len(trimedLine) > 0 && trimedLine[0] != ':' {
			lines = append(lines, trimedLine)
		}
		if err != nil {
			if err == io.EOF {
				s.isFinished = true
				break
			}
			return nil, err
		}

		if string(line) == "\n" {
			break
		}
	}

	if len(lines) == 0 {
		if s.isFinished {
			return nil, io.EOF
		} else {
			return nil, fmt.Errorf("no content")
		}
	}

	rsp := new(T)

	if err := json.Unmarshal(bytes.Join(lines, []byte("\n")), rsp); err == nil {
		return rsp, nil
	}

	mergedLines := map[string]string{}
	for _, line := range lines {
		items := strings.SplitN(string(line), ":", 2)
		if len(items) != 2 {
			return nil, errors.New("can not parse see response data")
		}
		if v, ok := mergedLines[items[0]]; ok {
			mergedLines[items[0]] = v + "\n" + items[1]
		} else {
			mergedLines[items[0]] = items[1]
		}
	}

	objLines := map[string]interface{}{}
	for k, v := range mergedLines {
		vv := map[string]interface{}{}
		if json.Unmarshal([]byte(v), &vv) == nil {
			objLines[k] = vv
		} else {
			objLines[k] = v
		}
	}

	jsonBytes, _ := json.Marshal(objLines)

	if err := json.Unmarshal(jsonBytes, rsp); err != nil {
		return nil, err
	}

	return rsp, nil
}

type SSEJsonResponse[T any] struct {
	decoder    *json.Decoder
	response   *http.Response
	isFinished bool
	isReading  bool
}

func NewSSEJsonResponse[T any](rsp *http.Response) *SSEJsonResponse[T] {
	return &SSEJsonResponse[T]{
		decoder:  json.NewDecoder(rsp.Body),
		response: rsp,
	}
}

func (s *SSEJsonResponse[T]) Recv() (response T, err error) {
	if s.isFinished {
		err = io.EOF
		return
	}

	return s.process()
}

func (s *SSEJsonResponse[T]) Close() {
	s.response.Body.Close()
}

func (s *SSEJsonResponse[T]) process() (response T, err error) {

	if !s.isReading {
		_, err = s.decoder.Token()
		if err != nil {
			return
		}
		s.isReading = true
	}

	if s.decoder.More() {
		err = s.decoder.Decode(&response)
		return
	}

	// read closing bracket
	_, err = s.decoder.Token()
	if err != nil {
		return
	}

	s.isFinished = true
	err = io.EOF
	return
}
