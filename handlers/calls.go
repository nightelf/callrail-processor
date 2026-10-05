package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

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

	result, err := persistence.UpsertCall(r.Context(), document)
	if err != nil {
		log.Printf("failed to upsert call %s: %v", id, err)
		http.Error(w, "Failed to save call", http.StatusInternalServerError)
		return
	}

	status := http.StatusOK
	if result == persistence.Inserted {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"status": "success", "result": result, "id": id})
}

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// GET /api/calls?page=1&limit=50
func ListCalls(w http.ResponseWriter, r *http.Request) {
	page, err := queryInt(r, "page", 1)
	if err != nil || page < 1 {
		http.Error(w, "page must be a whole number of 1 or more", http.StatusBadRequest)
		return
	}
	limit, err := queryInt(r, "limit", defaultPageSize)
	if err != nil || limit < 1 || limit > maxPageSize {
		http.Error(w, fmt.Sprintf("limit must be a whole number from 1 to %d", maxPageSize), http.StatusBadRequest)
		return
	}

	calls, total, err := persistence.ListCalls(r.Context(), page, limit)
	if err != nil {
		log.Printf("failed to list calls: %v", err)
		http.Error(w, "Failed to load calls", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":        calls,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": (total + limit - 1) / limit,
	})
}

// queryInt reads an integer query parameter, or returns fallback if it's absent.
func queryInt(r *http.Request, name string, fallback int64) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
