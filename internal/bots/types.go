package bots

// Platform identifies the messaging platform.
type Platform string

const (
	PlatformSlack Platform = "slack"
	PlatformTeams Platform = "teams"
)

// IncomingMessage represents a message received from any platform.
type IncomingMessage struct {
	Platform  Platform
	ChannelID string
	UserID    string
	UserName  string
	Text      string
	ThreadID  string // for threaded replies
	Timestamp string
}

// OutgoingMessage represents a response to send back.
type OutgoingMessage struct {
	ChannelID string
	Text      string
	ThreadID  string
	Blocks    string // JSON blocks for Slack Block Kit / Teams Adaptive Cards
}

// BotConfig holds bot configuration.
type BotConfig struct {
	Platform      Platform
	Token         string
	SigningSecret string
	WebhookURL    string // for Teams
}
