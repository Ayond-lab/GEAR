package pepcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPExtractor struct {
	BaseURL string
	Client  *http.Client
}

func NewHTTPExtractor(baseURL string) *HTTPExtractor {
	return &HTTPExtractor{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (e *HTTPExtractor) Extract(ctx context.Context, _ ActiveAction, request ExtractRequest) (ExtractResult, error) {
	if e == nil || strings.TrimSpace(e.BaseURL) == "" {
		return ExtractResult{}, ErrExtractorUnavailable
	}
	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	body, err := json.Marshal(request)
	if err != nil {
		return ExtractResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, e.BaseURL+"/v1/extract", bytes.NewReader(body))
	if err != nil {
		return ExtractResult{}, err
	}
	httpRequest.Header.Set("content-type", "application/json")

	resp, err := client.Do(httpRequest)
	if err != nil {
		return ExtractResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return ExtractResult{}, fmt.Errorf("extractor status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result ExtractResult
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return ExtractResult{}, err
	}
	return result, nil
}
