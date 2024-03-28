package message

type CompletionsRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	MaxTokens   int      `json:"max_tokens"`
	Temperature float32  `json:"temperature"`
	TopP        float32  `json:"top_p"`
	Stop        []string `json:"stop"`
	Stream      bool     `json:"stream"`
}

type CompletionsResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []CompletionsChoice `json:"choices"`
	Usage   *CompletionsUsage   `json:"usage,omitempty"`
}

type CompletionsChoice struct {
	Index        int     `json:"index"`
	FinishReason *string `json:"finish_reason"`
	Text         string  `json:"text"`
}

type CompletionsUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ListModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type Model struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	OwnedBy string `json:"owned_by"`
}
