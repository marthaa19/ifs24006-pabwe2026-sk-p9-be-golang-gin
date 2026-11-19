package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Completer mengirim percakapan ke model bahasa dan mengembalikan teks jawabannya.
type Completer interface {
	Complete(messages []Message) (string, error)
}

// OpenAIClient memanggil <OPENAI_ENDPOINT>/chat/completions.
type OpenAIClient struct {
	Endpoint string
	APIKey   string
	Model    string
	Client   *http.Client
}

// NewOpenAIClient membuat OpenAIClient dengan timeout 90 detik.
func NewOpenAIClient(endpoint, apiKey, model string) *OpenAIClient {
	return &OpenAIClient{Endpoint: endpoint, APIKey: apiKey, Model: model, Client: &http.Client{Timeout: 90 * time.Second}}
}

func (c *OpenAIClient) url() string {
	u := strings.TrimRight(c.Endpoint, "/")
	if !strings.HasSuffix(u, "/chat/completions") {
		u += "/chat/completions"
	}
	return u
}

// Complete mengirim permintaan chat completions.
func (c *OpenAIClient) Complete(messages []Message) (string, error) {
	payload, _ := json.Marshal(chatRequest{Model: c.Model, Messages: messages, Temperature: 0.7})
	req, err := http.NewRequest(http.MethodPost, c.url(), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal menghubungi layanan AI: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("layanan AI membalas status %d", resp.StatusCode)
	}
	var out chatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("balasan layanan AI bukan JSON: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("layanan AI error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("layanan AI tidak memberi jawaban")
	}
	return out.Choices[0].Message.Content, nil
}
