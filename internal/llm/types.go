package llm

// Role represents the role of a message sender in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message represents a single message in a conversation.
type Message struct {
	Role    Role
	Content string
}

// CompletionRequest contains the parameters for an LLM completion request.
type CompletionRequest struct {
	Model       string
	Messages    []Message
	MaxTokens   int
	Temperature float64
	JSONMode    bool
}

// CompletionResponse contains the result of an LLM completion request.
type CompletionResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
	Model        string
	FinishReason string
}
