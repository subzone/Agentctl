package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Executor runs a task (the incoming user text) and returns the agent's
// reply. The server is decoupled from the engine through this func, so it can
// be tested with a stub and reused by any caller that can run a task.
type Executor func(ctx context.Context, task string) (string, error)

// Server serves a single agent's A2A card and message endpoint.
type Server struct {
	card AgentCard
	exec Executor
}

// NewServer builds an A2A server for one agent.
func NewServer(card AgentCard, exec Executor) *Server {
	return &Server{card: card, exec: exec}
}

// Handler returns an http.Handler exposing:
//   - GET /.well-known/agent.json (and /agent-card.json) → the Agent Card
//   - GET /health → liveness
//   - POST / → JSON-RPC 2.0 (method "message/send")
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent.json", s.handleCard)
	mux.HandleFunc("/.well-known/agent-card.json", s.handleCard)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/", s.handleRPC)
	return mux
}

func (s *Server) handleCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.card)
}

func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, nil, codeParseError, "parse error")
		return
	}
	if req.JSONRPC != "2.0" {
		writeRPCError(w, req.ID, codeInvalidRequest, "jsonrpc must be \"2.0\"")
		return
	}

	switch req.Method {
	case "message/send":
		s.handleSend(w, r.Context(), req)
	default:
		writeRPCError(w, req.ID, codeMethodNotFound, "unknown method: "+req.Method)
	}
}

func (s *Server) handleSend(w http.ResponseWriter, ctx context.Context, req rpcRequest) {
	var p sendParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		writeRPCError(w, req.ID, codeInvalidParams, "invalid params")
		return
	}
	task := p.Message.Text()
	if task == "" {
		writeRPCError(w, req.ID, codeInvalidParams, "message has no text content")
		return
	}

	reply, err := s.exec(ctx, task)
	if err != nil {
		writeRPCError(w, req.ID, codeInternalError, err.Error())
		return
	}

	result := TextMessage("agent", fmt.Sprintf("msg_%d", time.Now().UnixNano()), reply)
	writeJSON(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	// JSON-RPC errors are still delivered with HTTP 200 per convention.
	writeJSON(w, http.StatusOK, rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg},
	})
}
