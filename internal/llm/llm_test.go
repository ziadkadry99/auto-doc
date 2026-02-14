package llm

import (
	"context"
	"sync"
	"testing"
	"time"
)

// MockProvider is a test provider that records calls and returns canned responses.
type MockProvider struct {
	mu        sync.Mutex
	Calls     []CompletionRequest
	Response  *CompletionResponse
	Err       error
	ProvName  string
}

func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		ProvName: name,
		Response: &CompletionResponse{
			Content:      "mock response",
			InputTokens:  10,
			OutputTokens: 20,
			Model:        "mock-model",
			FinishReason: "stop",
		},
	}
}

func (m *MockProvider) Name() string {
	return m.ProvName
}

func (m *MockProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, req)
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Response, nil
}

func (m *MockProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Calls)
}

// --- Tests ---

func TestMockProviderRecordsCalls(t *testing.T) {
	mock := NewMockProvider("test")
	ctx := context.Background()

	req := CompletionRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	}

	resp, err := mock.Complete(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "mock response" {
		t.Errorf("expected 'mock response', got %q", resp.Content)
	}

	if mock.CallCount() != 1 {
		t.Errorf("expected 1 call, got %d", mock.CallCount())
	}

	if mock.Calls[0].Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", mock.Calls[0].Model)
	}
}

func TestFactoryReturnsErrorForMissingAPIKey(t *testing.T) {
	// Ensure env vars are not set for this test.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	providers := []string{"anthropic", "openai", "google"}
	for _, p := range providers {
		_, err := NewProvider(p, "some-model")
		if err == nil {
			t.Errorf("expected error for provider %q with missing API key", p)
		}
	}
}

func TestFactoryReturnsErrorForUnknownProvider(t *testing.T) {
	_, err := NewProvider("unknown", "some-model")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestFactoryCreatesOllamaWithoutAPIKey(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://localhost:11434")
	provider, err := NewProvider("ollama", "llama3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.Name() != "ollama" {
		t.Errorf("expected name 'ollama', got %q", provider.Name())
	}
}

func TestFactoryCreatesOllamaWithDefaultHost(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "")
	provider, err := NewProvider("ollama", "llama3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ollamaP, ok := provider.(*OllamaProvider)
	if !ok {
		t.Fatal("expected *OllamaProvider")
	}
	if ollamaP.baseURL != "http://localhost:11434" {
		t.Errorf("expected default host, got %q", ollamaP.baseURL)
	}
}

func TestFactoryCreatesAnthropicProvider(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	provider, err := NewProvider("anthropic", "claude-sonnet-4-5-20250929")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got %q", provider.Name())
	}
}

func TestFactoryCreatesOpenAIProvider(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	provider, err := NewProvider("openai", "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.Name() != "openai" {
		t.Errorf("expected name 'openai', got %q", provider.Name())
	}
}

func TestFactoryCreatesGoogleProvider(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "test-key")
	provider, err := NewProvider("google", "gemini-2.0-flash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.Name() != "google" {
		t.Errorf("expected name 'google', got %q", provider.Name())
	}
}

func TestRateLimiterPassesThrough(t *testing.T) {
	mock := NewMockProvider("test")
	rl := NewRateLimitedProvider(mock, 60)

	ctx := context.Background()
	req := CompletionRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	}

	resp, err := rl.Complete(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "mock response" {
		t.Errorf("expected 'mock response', got %q", resp.Content)
	}
	if rl.Name() != "test" {
		t.Errorf("expected name 'test', got %q", rl.Name())
	}
}

func TestRateLimiterLimitsRequests(t *testing.T) {
	mock := NewMockProvider("test")
	// Allow only 2 requests per minute.
	rl := NewRateLimitedProvider(mock, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := CompletionRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	}

	// First two should succeed immediately.
	for i := 0; i < 2; i++ {
		_, err := rl.Complete(ctx, req)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}

	// Third should block and eventually fail due to context timeout.
	_, err := rl.Complete(ctx, req)
	if err == nil {
		t.Error("expected error due to rate limiting + context timeout")
	}
}

func TestEstimateCostKnownModels(t *testing.T) {
	tests := []struct {
		model        string
		inputTokens  int
		outputTokens int
		wantMin      float64
	}{
		{"claude-sonnet-4-5-20250929", 1000, 500, 0.0},
		{"gpt-4o", 1000, 500, 0.0},
		{"gemini-2.0-flash", 1000, 500, 0.0},
	}

	for _, tt := range tests {
		cost := EstimateCost(tt.model, tt.inputTokens, tt.outputTokens)
		if cost <= tt.wantMin {
			t.Errorf("EstimateCost(%q, %d, %d) = %f, expected > %f",
				tt.model, tt.inputTokens, tt.outputTokens, cost, tt.wantMin)
		}
	}
}

func TestEstimateCostUnknownModel(t *testing.T) {
	cost := EstimateCost("unknown-model", 1000, 500)
	if cost != 0 {
		t.Errorf("expected 0 for unknown model, got %f", cost)
	}
}

func TestEstimateCostAccuracy(t *testing.T) {
	// claude-sonnet-4-5: $3/1M input, $15/1M output
	// 1M input + 1M output = $3 + $15 = $18
	cost := EstimateCost("claude-sonnet-4-5-20250929", 1_000_000, 1_000_000)
	expected := 18.0
	if cost < expected-0.01 || cost > expected+0.01 {
		t.Errorf("expected cost ~$%.2f, got $%.2f", expected, cost)
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"hi", 1},
		{"hello world!!", 3},
		{"a longer piece of text that has more characters", 11},
	}

	for _, tt := range tests {
		got := EstimateTokens(tt.text)
		if got != tt.want {
			t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestRoles(t *testing.T) {
	if RoleSystem != "system" {
		t.Errorf("RoleSystem = %q, want 'system'", RoleSystem)
	}
	if RoleUser != "user" {
		t.Errorf("RoleUser = %q, want 'user'", RoleUser)
	}
	if RoleAssistant != "assistant" {
		t.Errorf("RoleAssistant = %q, want 'assistant'", RoleAssistant)
	}
}
