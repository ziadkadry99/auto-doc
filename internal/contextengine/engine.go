package contextengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ziadkadry99/auto-doc/internal/llm"
)

// Engine is the conversational context engine that processes natural language
// input and extracts structured facts for documentation.
type Engine struct {
	store       *Store
	llmProvider llm.Provider
	llmModel    string
}

// NewEngine creates a new context engine.
func NewEngine(store *Store, provider llm.Provider, model string) *Engine {
	return &Engine{
		store:       store,
		llmProvider: provider,
		llmModel:    model,
	}
}

// ProcessInput takes natural language user input and extracts structured facts,
// determines affected documentation, and generates clarification questions if needed.
func (e *Engine) ProcessInput(ctx context.Context, sessionID, userID, input string) (*ContextUpdate, error) {
	// Get existing facts for context.
	existingFacts, err := e.store.GetCurrentFacts(ctx, "", "", "")
	if err != nil {
		return nil, fmt.Errorf("loading existing facts: %w", err)
	}

	// Get conversation history for context.
	var history []ConversationMessage
	if sessionID != "" {
		history, err = e.store.GetMessages(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("loading conversation history: %w", err)
		}
	}

	// Build LLM prompt.
	prompt := buildExtractionPrompt(input, existingFacts, history)

	resp, err := e.llmProvider.Complete(ctx, llm.CompletionRequest{
		Model: e.llmModel,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   4096,
		Temperature: 0.2,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM completion: %w", err)
	}

	// Parse the LLM response.
	update, err := parseExtractionResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parsing LLM response: %w", err)
	}

	// Save extracted facts.
	for _, ef := range update.Facts {
		fact := Fact{
			Scope:      ef.Scope,
			ScopeID:    ef.ScopeID,
			Key:        ef.Key,
			Value:      ef.Value,
			Source:     "user",
			ProvidedBy: userID,
		}
		if _, err := e.store.SaveFact(ctx, fact); err != nil {
			return nil, fmt.Errorf("saving fact: %w", err)
		}
	}

	// Store the conversation messages.
	if sessionID != "" {
		e.store.AddMessage(ctx, ConversationMessage{
			SessionID: sessionID,
			Role:      "user",
			Content:   input,
		})
		e.store.AddMessage(ctx, ConversationMessage{
			SessionID: sessionID,
			Role:      "assistant",
			Content:   update.Summary,
		})
	}

	return update, nil
}

// AskQuestion asks the engine a free-form question about the architecture.
func (e *Engine) AskQuestion(ctx context.Context, question string) (string, error) {
	// Get all current facts for context.
	facts, err := e.store.GetCurrentFacts(ctx, "", "", "")
	if err != nil {
		return "", fmt.Errorf("loading facts: %w", err)
	}

	prompt := buildQuestionPrompt(question, facts)

	resp, err := e.llmProvider.Complete(ctx, llm.CompletionRequest{
		Model: e.llmModel,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: questionSystemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   2048,
		Temperature: 0.3,
	})
	if err != nil {
		return "", fmt.Errorf("LLM completion: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

// ProcessCorrection handles a user correcting a previously stored fact.
func (e *Engine) ProcessCorrection(ctx context.Context, userID string, correction Correction) (*ContextUpdate, error) {
	// Get the original fact.
	original, err := e.store.GetFact(ctx, correction.OriginalFactID)
	if err != nil {
		return nil, fmt.Errorf("getting original fact: %w", err)
	}
	if original == nil {
		return nil, fmt.Errorf("fact not found: %s", correction.OriginalFactID)
	}

	// Save the corrected fact (supersedes the original).
	corrected := Fact{
		RepoID:     original.RepoID,
		Scope:      original.Scope,
		ScopeID:    original.ScopeID,
		Key:        original.Key,
		Value:      correction.NewValue,
		Source:     "user",
		ProvidedBy: userID,
	}

	saved, err := e.store.SaveFact(ctx, corrected)
	if err != nil {
		return nil, fmt.Errorf("saving corrected fact: %w", err)
	}

	return &ContextUpdate{
		Facts: []ExtractedFact{{
			Scope:      saved.Scope,
			ScopeID:    saved.ScopeID,
			Key:        saved.Key,
			Value:      saved.Value,
			Confidence: "high",
			Explanation: fmt.Sprintf("Corrected by %s: %s", userID, correction.Reason),
		}},
		Summary: fmt.Sprintf("Corrected %s.%s for %s: %q → %q",
			original.Scope, original.Key, original.ScopeID,
			original.Value, correction.NewValue),
	}, nil
}

// Store returns the underlying store for direct access.
func (e *Engine) Store() *Store {
	return e.store
}

const systemPrompt = `You are a documentation context extraction engine. Your job is to analyze user input about software architecture and extract structured facts.

You MUST respond with valid JSON matching this schema:
{
  "facts": [
    {
      "scope": "service|endpoint|flow|org|domain|topic",
      "scope_id": "identifier (e.g. service name)",
      "key": "description|purpose|owner|dependency|technology|protocol|data_flow|business_context",
      "value": "the extracted fact",
      "confidence": "high|medium|low",
      "explanation": "why you extracted this"
    }
  ],
  "clarifications": [
    {
      "question": "follow-up question if input is ambiguous",
      "options": ["option1", "option2"],
      "context": "why you need this clarified"
    }
  ],
  "affected_docs": ["list of service/component names whose docs should be updated"],
  "summary": "brief summary of what was understood and what changes were made"
}

Rules:
- Extract ALL facts from the input, even if there are many
- Use specific scope_ids (actual service/component names mentioned)
- If the user mentions relationships between services, create facts for both sides
- If something is ambiguous, add a clarification question
- Be aggressive about extracting useful information
- The summary should confirm what you understood back to the user`

const questionSystemPrompt = `You are an architecture documentation assistant. Answer questions about the software architecture based on the known facts provided. Be specific, reference actual service names and relationships. If you don't have enough information to answer fully, say what you do know and what's missing.`

func buildExtractionPrompt(input string, existingFacts []Fact, history []ConversationMessage) string {
	var b strings.Builder

	b.WriteString("## Existing Knowledge\n")
	if len(existingFacts) > 0 {
		for _, f := range existingFacts {
			fmt.Fprintf(&b, "- [%s] %s.%s = %s\n", f.Scope, f.ScopeID, f.Key, f.Value)
		}
	} else {
		b.WriteString("(No existing facts yet)\n")
	}

	if len(history) > 0 {
		b.WriteString("\n## Recent Conversation\n")
		for _, m := range history {
			if len(history) > 10 {
				// Only show last 10 messages for context window management.
				history = history[len(history)-10:]
			}
			fmt.Fprintf(&b, "%s: %s\n", m.Role, m.Content)
		}
	}

	fmt.Fprintf(&b, "\n## New User Input\n%s\n", input)
	fmt.Fprintf(&b, "\nExtract all facts from this input. Consider the existing knowledge to avoid duplicates and detect corrections or updates.")

	return b.String()
}

func buildQuestionPrompt(question string, facts []Fact) string {
	var b strings.Builder

	b.WriteString("## Known Architecture Facts\n")
	if len(facts) > 0 {
		for _, f := range facts {
			fmt.Fprintf(&b, "- [%s] %s.%s = %s (source: %s)\n", f.Scope, f.ScopeID, f.Key, f.Value, f.Source)
		}
	} else {
		b.WriteString("(No facts available yet)\n")
	}

	fmt.Fprintf(&b, "\n## Question\n%s\n", question)

	return b.String()
}

func parseExtractionResponse(content string) (*ContextUpdate, error) {
	// Try to find JSON in the response (may be wrapped in markdown code blocks).
	jsonStr := content
	if idx := strings.Index(content, "{"); idx >= 0 {
		jsonStr = content[idx:]
	}
	if idx := strings.LastIndex(jsonStr, "}"); idx >= 0 {
		jsonStr = jsonStr[:idx+1]
	}

	var update ContextUpdate
	if err := json.Unmarshal([]byte(jsonStr), &update); err != nil {
		// If JSON parsing fails, return a basic update with the raw content as summary.
		return &ContextUpdate{
			Summary: content,
		}, nil
	}

	return &update, nil
}
