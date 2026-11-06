package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

const endpoint = "https://generativelanguage.googleapis.com/v1beta/models/"

type Client struct {
	apiKey      string
	llmModel    string
	embedding   string
	dimensions  int
	http        *http.Client
	retryDelays []time.Duration
}

func New(apiKey, llmModel, embeddingModel string, dimensions int) *Client {
	return &Client{
		apiKey: apiKey, llmModel: llmModel, embedding: embeddingModel, dimensions: dimensions,
		http:        &http.Client{Timeout: 35 * time.Second},
		retryDelays: []time.Duration{time.Second, 2 * time.Second},
	}
}

func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body := map[string]any{
		"content":  map[string]any{"parts": []any{map[string]string{"text": text}}},
		"taskType": "RETRIEVAL_QUERY", "outputDimensionality": c.dimensions,
	}
	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := c.call(ctx, c.embedding+":embedContent", body, &result); err != nil {
		return nil, err
	}
	if len(result.Embedding.Values) != c.dimensions {
		return nil, fmt.Errorf("Gemini embedding dimension mismatch: got %d, want %d", len(result.Embedding.Values), c.dimensions)
	}
	return result.Embedding.Values, nil
}

type answerSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]schemaField `json:"properties"`
	Required   []string               `json:"required"`
}

type schemaField struct {
	Type  string `json:"type"`
	Items *struct {
		Type string `json:"type"`
	} `json:"items,omitempty"`
}

func (c *Client) GenerateAnswer(ctx context.Context, system, prompt string) (bool, string, []int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	items := &struct {
		Type string `json:"type"`
	}{Type: "INTEGER"}
	schema := answerSchema{
		Type: "OBJECT",
		Properties: map[string]schemaField{
			"found":         {Type: "BOOLEAN"},
			"answer":        {Type: "STRING"},
			"used_excerpts": {Type: "ARRAY", Items: items},
		},
		Required: []string{"found", "answer", "used_excerpts"},
	}
	body := map[string]any{
		"systemInstruction": map[string]any{"parts": []any{map[string]string{"text": system}}},
		"contents":          []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": prompt}}}},
		"generationConfig": map[string]any{
			"temperature":      0,
			"responseMimeType": "application/json",
			"responseSchema":   schema,
		},
	}
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := c.call(ctx, c.llmModel+":generateContent", body, &response); err != nil {
		return false, "", nil, err
	}
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return false, "", nil, errors.New("Gemini returned no candidate text")
	}
	var parsed struct {
		Found        bool   `json:"found"`
		Answer       string `json:"answer"`
		UsedExcerpts []int  `json:"used_excerpts"`
	}
	if err := json.Unmarshal([]byte(response.Candidates[0].Content.Parts[0].Text), &parsed); err != nil {
		return false, "", nil, fmt.Errorf("decode Gemini structured answer: %w", err)
	}
	return parsed.Found, parsed.Answer, parsed.UsedExcerpts, nil
}

func (c *Client) call(ctx context.Context, operation string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := endpoint + operation
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", c.apiKey)
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
		} else {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("Gemini status %s: %s", resp.Status, string(data))
			} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return fmt.Errorf("Gemini request rejected (%s): %s", resp.Status, string(data))
			} else if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("decode Gemini response: %w", err)
			} else {
				return nil
			}
		}
		if attempt < 2 {
			delay := c.retryDelays[attempt]
			if lastErr != nil {
				// Deterministic bounded exponential backoff keeps API latency predictable.
				delay = time.Duration(math.Min(float64(delay), float64(4*time.Second)))
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return fmt.Errorf("Gemini retries exhausted: %w", lastErr)
}

// ParseDimensions accepts the environment value without silently rounding it.
func ParseDimensions(value string) (int, error) {
	dimensions, err := strconv.Atoi(value)
	if err != nil || dimensions <= 0 {
		return 0, fmt.Errorf("invalid embedding dimension %q", value)
	}
	return dimensions, nil
}
