package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"example.com/solo-0021-metrology-assurance-suite/internal/bootstrap"
)

const requestTimeout = 5 * time.Second

// apiResponse is the decoded result of a single harness request.
type apiResponse struct {
	Status int
	Body   map[string]any
	Raw    string
}

// harness owns the in-process HTTP server, its client, and every request
// made against the service. Scenarios must use request instead of touching
// the client directly so connection cleanup and response decoding stay in
// one place.
type harness struct {
	baseURL string
	client  *http.Client
	server  *httptest.Server
}

func newHarness() *harness {
	server := httptest.NewServer(bootstrap.NewHandler())
	return &harness{
		baseURL: server.URL,
		client:  server.Client(),
		server:  server,
	}
}

func (h *harness) close() {
	h.server.Close()
}

// request sends one JSON request and fully reads and decodes the response.
// Body close, the per-request timeout, and decode-error attribution live
// only here.
func (h *harness) request(method, path string, body any) (apiResponse, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return apiResponse{}, err
		}
		reader = bytes.NewReader(encoded)
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, reader)
	if err != nil {
		return apiResponse{}, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := h.client.Do(request)
	if err != nil {
		return apiResponse{}, err
	}
	defer response.Body.Close()
	rawBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return apiResponse{}, err
	}
	result := apiResponse{Status: response.StatusCode, Raw: string(rawBytes)}
	if len(bytes.TrimSpace(rawBytes)) > 0 {
		if err := json.Unmarshal(rawBytes, &result.Body); err != nil {
			return apiResponse{}, fmt.Errorf("decode %s %s response: %w", method, path, err)
		}
	}
	return result, nil
}

// requestResult pairs a concurrent attempt with its transport error.
type requestResult struct {
	response apiResponse
	err      error
}

// race runs count HTTP requests concurrently and returns their outcomes in
// index order. attempt builds the request for the given attempt index. The
// goroutine fan-out and completion handshake stay here; scenarios own the
// interpretation of the returned outcomes.
func (h *harness) race(count int, attempt func(index int) (apiResponse, error)) []requestResult {
	results := make([]requestResult, count)
	var wait sync.WaitGroup
	for index := 0; index < count; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			response, err := attempt(index)
			results[index] = requestResult{response: response, err: err}
		}(index)
	}
	wait.Wait()
	return results
}
