package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"callprocessor/persistence"
)

func CreateCall(w http.ResponseWriter, r *http.Request) {
	var document map[string]any
	if err := json.NewDecoder(r.Body).Decode(&document); err != nil {
		log.Printf("invalid JSON payload: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	id, ok := document["id"].(string)
	if !ok || id == "" {
		http.Error(w, "Missing call id", http.StatusBadRequest)
		return
	}

	created, err := persistence.UpsertCall(r.Context(), document)
	if err != nil {
		log.Printf("failed to upsert call %s: %v", id, err)
		http.Error(w, "Failed to save call", http.StatusInternalServerError)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"status": "success", "id": id})
}

func ListCalls(w http.ResponseWriter, r *http.Request) {
	calls, err := persistence.ListCalls(r.Context())
	if err != nil {
		log.Printf("failed to list calls: %v", err)
		http.Error(w, "Failed to load calls", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, calls)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
