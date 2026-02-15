package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/ziadkadry99/auto-doc/internal/backlog"
	"github.com/ziadkadry99/auto-doc/internal/contextengine"
)

// statsResponse is the JSON response for the stats endpoint.
type statsResponse struct {
	TotalFacts    int `json:"total_facts"`
	OpenQuestions int `json:"open_questions"`
	TotalSessions int `json:"total_sessions"`
}

// recentResponse is the JSON response for the recent activity endpoint.
type recentResponse struct {
	Facts     []contextengine.Fact `json:"facts"`
	Questions []backlog.Question   `json:"questions"`
}

func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	facts, err := d.engine.Store().GetCurrentFacts(ctx, "", "", "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	openCount := 0
	if d.backlogStore != nil {
		openCount, err = d.backlogStore.GetOpenCount(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	totalSessions, _ := d.engine.Store().CountSessions(ctx)

	writeJSON(w, http.StatusOK, statsResponse{
		TotalFacts:    len(facts),
		OpenQuestions: openCount,
		TotalSessions: totalSessions,
	})
}

func (d *Dashboard) handleRecent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get recent facts (all current, limited to 10).
	facts, err := d.engine.Store().GetCurrentFacts(ctx, "", "", "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if len(facts) > 10 {
		facts = facts[:10]
	}

	// Get recent questions.
	var questions []backlog.Question
	if d.backlogStore != nil {
		questions, err = d.backlogStore.List(ctx, backlog.ListFilter{Limit: 10})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if facts == nil {
		facts = []contextengine.Fact{}
	}
	if questions == nil {
		questions = []backlog.Question{}
	}

	writeJSON(w, http.StatusOK, recentResponse{
		Facts:     facts,
		Questions: questions,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
