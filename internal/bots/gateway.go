package bots

import "context"

// MessageHandler processes incoming messages and produces responses.
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg IncomingMessage) (*OutgoingMessage, error)
}

// Gateway is the platform-agnostic bot gateway that routes messages
// to a handler for processing.
type Gateway struct {
	handler MessageHandler
}

// NewGateway creates a new Gateway with the given message handler.
func NewGateway(handler MessageHandler) *Gateway {
	return &Gateway{handler: handler}
}

// Process routes an incoming message through the handler.
func (g *Gateway) Process(ctx context.Context, msg IncomingMessage) (*OutgoingMessage, error) {
	return g.handler.HandleMessage(ctx, msg)
}
