package llm

import (
	"github.com/tmc/langchaingo/llms/mistral"
)

// Client wraps a single Mistral model connection and implements
// inbox.LLMClient. Constructed once at startup rather than per call so the
// underlying connection is reused, and so it can be injected/mocked in tests.
type Client struct {
	llm *mistral.Model
}

func NewClient() (*Client, error) {
	m, err := mistral.New(mistral.WithModel("mistral-small-latest"))
	if err != nil {
		return nil, err
	}
	return &Client{llm: m}, nil
}
