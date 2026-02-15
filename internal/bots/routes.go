package bots

import "github.com/go-chi/chi/v5"

// RegisterRoutes mounts the bot webhook endpoints on the given router.
func RegisterRoutes(r chi.Router, slackHandler *SlackHandler, teamsHandler *TeamsHandler) {
	r.Post("/api/bots/slack/events", slackHandler.HandleEvent)
	r.Post("/api/bots/teams/activity", teamsHandler.HandleActivity)
}
