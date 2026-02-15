package bots

import (
	"context"
	"fmt"
	"strings"

	"github.com/ziadkadry99/auto-doc/internal/backlog"
	"github.com/ziadkadry99/auto-doc/internal/contextengine"
)

// Processor connects incoming bot messages to the context engine and backlog.
type Processor struct {
	ctxEngine    *contextengine.Engine
	backlogStore *backlog.Store
}

// NewProcessor creates a new message processor.
func NewProcessor(engine *contextengine.Engine, backlogStore *backlog.Store) *Processor {
	return &Processor{
		ctxEngine:    engine,
		backlogStore: backlogStore,
	}
}

// HandleMessage processes an incoming message and returns a response.
// It detects intent from the message text:
//   - "ask " or "?" prefix -> use engine.AskQuestion
//   - "context " or "info " prefix -> use engine.ProcessInput
//   - "questions" or "backlog" -> return top priority questions
//   - default -> use engine.ProcessInput (treat as context provision)
func (p *Processor) HandleMessage(ctx context.Context, msg IncomingMessage) (*OutgoingMessage, error) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return &OutgoingMessage{
			ChannelID: msg.ChannelID,
			ThreadID:  msg.ThreadID,
			Text:      "I received an empty message. Please provide some text.",
		}, nil
	}

	lower := strings.ToLower(text)

	var responseText string
	var err error

	switch {
	case strings.HasPrefix(lower, "ask "):
		question := text[4:]
		responseText, err = p.handleAsk(ctx, question)

	case strings.HasPrefix(lower, "?"):
		question := strings.TrimSpace(text[1:])
		responseText, err = p.handleAsk(ctx, question)

	case strings.HasPrefix(lower, "context "):
		input := text[8:]
		responseText, err = p.handleContext(ctx, msg, input)

	case strings.HasPrefix(lower, "info "):
		input := text[5:]
		responseText, err = p.handleContext(ctx, msg, input)

	case lower == "questions" || lower == "backlog":
		responseText, err = p.handleBacklog(ctx)

	default:
		responseText, err = p.handleContext(ctx, msg, text)
	}

	if err != nil {
		return &OutgoingMessage{
			ChannelID: msg.ChannelID,
			ThreadID:  msg.ThreadID,
			Text:      fmt.Sprintf("Error processing your message: %v", err),
		}, nil
	}

	return &OutgoingMessage{
		ChannelID: msg.ChannelID,
		ThreadID:  msg.ThreadID,
		Text:      responseText,
	}, nil
}

func (p *Processor) handleAsk(ctx context.Context, question string) (string, error) {
	if p.ctxEngine == nil {
		return "", fmt.Errorf("context engine not configured")
	}
	return p.ctxEngine.AskQuestion(ctx, question)
}

func (p *Processor) handleContext(ctx context.Context, msg IncomingMessage, input string) (string, error) {
	if p.ctxEngine == nil {
		return "", fmt.Errorf("context engine not configured")
	}
	sessionID := fmt.Sprintf("bot-%s-%s", msg.Platform, msg.ChannelID)
	update, err := p.ctxEngine.ProcessInput(ctx, sessionID, msg.UserID, input)
	if err != nil {
		return "", err
	}
	return update.Summary, nil
}

func (p *Processor) handleBacklog(ctx context.Context) (string, error) {
	if p.backlogStore == nil {
		return "", fmt.Errorf("backlog store not configured")
	}
	questions, err := p.backlogStore.GetTopPriority(ctx, 5)
	if err != nil {
		return "", err
	}
	if len(questions) == 0 {
		return "No open questions in the backlog.", nil
	}

	var b strings.Builder
	b.WriteString("Top priority questions:\n")
	for i, q := range questions {
		fmt.Fprintf(&b, "%d. [%s] (priority %d) %s\n", i+1, q.Category, q.Priority, q.Question)
	}
	return b.String(), nil
}
