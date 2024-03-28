package message

type CompletionsRequest struct {
	Model            string              `json:"model"`
	Prompt           string              `json:"prompt"`
	Echo             bool                `json:"echo,omitempty"`
	FrequencyPenalty *float32            `json:"frequency_penalty,omitempty"`
	LogitBias        *map[string]float32 `json:"logit_bias,omitempty"`
	Logprobs         *int                `json:"logprobs,omitempty"`
	MaxTokens        *int                `json:"max_tokens,omitempty"`
	N                *int                `json:"n,omitempty"`
	PresencePenalty  *float32            `json:"presence_penalty,omitempty"`
	Seed             *int                `json:"seed,omitempty"`
	Stop             *interface{}        `json:"stop,omitempty"`
	Stream           bool                `json:"stream"`
	Suffix           *string             `json:"suffix,omitempty"`
	Temperature      *float32            `json:"temperature,omitempty"`
	TopP             *float32            `json:"top_p,omitempty"`
	User             string              `json:"user,omitempty"`
}

type CompletionsResponse struct {
	ID                string              `json:"id"`
	Object            string              `json:"object"`
	Created           int64               `json:"created"`
	Model             string              `json:"model"`
	SystemFingerprint *string             `json:"system_fingerprint,omitempty"`
	Choices           []CompletionsChoice `json:"choices"`
	Usage             *CompletionsUsage   `json:"usage,omitempty"`
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
