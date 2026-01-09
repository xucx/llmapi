package httpx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func Do(client *http.Client, req *http.Request) (*http.Response, error) {
	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http return code %d", rsp.StatusCode)
	}

	return rsp, nil
}

func Get(client *http.Client, url string, header http.Header) (*http.Response, error) {

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	rsp, err := Do(client, req)
	if err != nil {
		return nil, err
	}

	return rsp, nil
}

func GetJSON[T any](client *http.Client, url string, header http.Header) (*T, error) {
	rsp, err := Get(client, url, header)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	rspData := new(T)
	if err := json.Unmarshal(body, rspData); err != nil {
		return nil, err
	}

	return rspData, nil
}

func Post(client *http.Client, url string, header http.Header, data interface{}) (*http.Response, error) {

	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	rsp, err := Do(client, req)
	if err != nil {
		return nil, err
	}

	return rsp, nil
}

func PostJson[T any](client *http.Client, url string, header http.Header, data interface{}) (*T, error) {
	rsp, err := Post(client, url, header, data)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	rspData := new(T)
	if err := json.Unmarshal(body, rspData); err != nil {
		return nil, err
	}

	return rspData, nil
}

func PostEventStream[T any](client *http.Client, url string, header http.Header, data interface{}) (*SSEEventResponse[T], error) {
	if header == nil {
		header = http.Header{}
	}
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")

	rsp, err := Post(client, url, header, data)
	if err != nil {
		return nil, err
	}

	return NewSSEEventResponse[T](rsp), nil
}

func PostJsonStream[T any](client *http.Client, url string, header http.Header, data interface{}) (*SSEJsonResponse[T], error) {
	header.Set("Content-Type", "application/json")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")

	rsp, err := Post(client, url, header, data)
	if err != nil {
		return nil, err
	}

	return NewSSEJsonResponse[T](rsp), nil
}
