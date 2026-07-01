package desktop

import (
	"context"
	"fmt"
	"time"

	"github.com/subzone/Agentctl/internal/llm"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ResetSession clears in-memory chat history for a session.
func (a *App) ResetSession(sessionID string) error {
	a.mu.RLock()
	sess, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	sess.mu.Lock()
	sess.engine.Reset()
	sess.Messages = nil
	sess.mu.Unlock()
	return nil
}

// RetryLastMessage re-runs the engine step with the last user message.
func (a *App) RetryLastMessage(sessionID string) error {
	a.mu.RLock()
	sess, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	sess.mu.Lock()
	msgs := sess.engine.Messages()
	var lastUser string
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != llm.RoleUser {
			continue
		}
		for _, b := range msgs[i].Content {
			if b.Type == llm.BlockText && b.Text != "" {
				lastUser = b.Text
				break
			}
		}
		if lastUser != "" {
			break
		}
	}
	if lastUser == "" {
		sess.mu.Unlock()
		return fmt.Errorf("nothing to retry")
	}
	sess.mu.Unlock()

	return a.runEngineStep(sessionID, lastUser, false)
}

// SwitchSessionModel changes the model for an active session.
func (a *App) SwitchSessionModel(sessionID, model string) error {
	if model == "" {
		return fmt.Errorf("model required")
	}
	a.mu.RLock()
	sess, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	provider, bare, err := llm.Resolve(model)
	if err != nil {
		return err
	}
	sess.mu.Lock()
	sess.engine.SetModel(provider, bare)
	sess.Model = model
	sess.mu.Unlock()
	return nil
}

func (a *App) runEngineStep(sessionID, message string, appendUser bool) error {
	a.mu.RLock()
	sess, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	sess.mu.Lock()
	if appendUser {
		sess.Messages = append(sess.Messages, Message{
			ID:        fmt.Sprintf("msg_%d", time.Now().UnixMilli()),
			Role:      "user",
			Content:   message,
			Timestamp: time.Now().Unix(),
		})
		runtime.EventsEmit(a.ctx, "message", map[string]any{
			"sessionId": sessionID,
			"role":      "user",
			"content":   message,
		})
	}
	sess.mu.Unlock()

	ctx, cancel := context.WithCancel(a.ctx)
	sess.mu.Lock()
	sess.cancel = cancel
	sess.mu.Unlock()

	go func() {
		defer cancel()
		sess.mu.Lock()
		engine := sess.engine
		model := sess.Model
		sess.mu.Unlock()

		if err := engine.Step(ctx, message); err != nil {
			runtime.EventsEmit(a.ctx, "error", map[string]any{
				"sessionId": sessionID,
				"error":     err.Error(),
			})
		}
		usage := engine.Usage()
		cost := calcCost(model, usage.InputTokens, usage.OutputTokens)
		runtime.EventsEmit(a.ctx, "done", map[string]any{
			"sessionId":    sessionID,
			"inputTokens":  usage.InputTokens,
			"outputTokens": usage.OutputTokens,
			"cost":         cost,
		})
		a.persistSessionToDisk(sess)
	}()
	return nil
}
