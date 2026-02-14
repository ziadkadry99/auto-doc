package main

import (
	"fmt"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users", handleUsers)
	mux.HandleFunc("/api/health", handleHealth)

	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", mux)
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Write([]byte(`[{"id": 1, "name": "Alice"}]`))
	case http.MethodPost:
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 2, "name": "Bob"}`))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status": "ok"}`))
}
