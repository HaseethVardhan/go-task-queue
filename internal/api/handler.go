package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HaseethVardhan/go-task-queue/internal/service"
	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

type TaskSubmitter interface {
	Submit(ctx context.Context, t task.Task) error
}

type Handler struct {
	taskService TaskSubmitter
}

func NewHandler(taskService TaskSubmitter) *Handler {
	if taskService == nil {
		panic("task service cannot be nil")
	}

	return &Handler{
		taskService: taskService,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /tasks", h.submitTask)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

type submitTaskRequest struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type submitTaskResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (h *Handler) submitTask(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB

	var req submitTaskRequest

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid JSON request body", http.StatusBadRequest)
		return
	}

	// Reject requests containing multiple JSON values.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "request body must contain a single JSON object", http.StatusBadRequest)
		return
	}

	req.Type = strings.TrimSpace(req.Type)
	if req.Type == "" {
		http.Error(w, "task type is required", http.StatusBadRequest)
		return
	}

	if len(req.Payload) == 0 {
		req.Payload = json.RawMessage(`{}`)
	}

	id, err := generateTaskID()
	if err != nil {
		http.Error(w, "failed to generate task ID", http.StatusInternalServerError)
		return
	}

	t := task.Task{
		ID:      id,
		Type:    req.Type,
		Payload: req.Payload,
	}

	if err := h.taskService.Submit(r.Context(), t); err != nil {
		if errors.Is(err, service.ErrInvalidTaskID) ||
			errors.Is(err, service.ErrInvalidTaskType) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(w, "failed to submit task", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusAccepted, submitTaskResponse{
		ID:     id,
		Status: "pending",
	})
}

func generateTaskID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	return "task_" + hex.EncodeToString(b[:]), nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}