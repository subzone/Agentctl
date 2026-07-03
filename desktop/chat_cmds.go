package desktop

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/subzone/Agentctl/internal/atfile"
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
	var lastImages []ImageAttachment
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != llm.RoleUser {
			continue
		}
		for _, b := range msgs[i].Content {
			switch b.Type {
			case llm.BlockText:
				if b.Text != "" {
					lastUser = b.Text
				}
			case llm.BlockImage:
				lastImages = append(lastImages, ImageAttachment{
					Name:     "image",
					MimeType: b.MimeType,
					DataB64:  base64.StdEncoding.EncodeToString(b.ImageData),
				})
			}
		}
		if lastUser != "" || len(lastImages) > 0 {
			break
		}
	}
	if lastUser == "" && len(lastImages) == 0 {
		sess.mu.Unlock()
		return fmt.Errorf("nothing to retry")
	}
	sess.mu.Unlock()

	return a.runEngineStep(sessionID, lastUser, lastImages, false)
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

func (a *App) runEngineStep(sessionID, message string, images []ImageAttachment, appendUser bool) error {
	a.mu.RLock()
	sess, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	expanded, _ := atfile.Expand(message)
	message = expanded

	blocks, err := buildUserBlocks(message, images)
	if err != nil {
		return err
	}

	display := message
	if len(images) > 0 {
		display += fmt.Sprintf(" [+%d image(s)]", len(images))
	}

	sess.mu.Lock()
	model := sess.Model
	if appendUser {
		sess.Messages = append(sess.Messages, Message{
			ID:        fmt.Sprintf("msg_%d", time.Now().UnixMilli()),
			Role:      "user",
			Content:   display,
			Timestamp: time.Now().Unix(),
		})
		runtime.EventsEmit(a.ctx, "message", map[string]any{
			"sessionId": sessionID,
			"role":      "user",
			"content":   display,
		})
	}
	if len(images) > 0 && !llm.SupportsVision(model) {
		runtime.EventsEmit(a.ctx, "error", map[string]any{
			"sessionId": sessionID,
			"error":     fmt.Sprintf("model %q may not support images — try gpt-4o, claude-sonnet-4-6, or gemini-2.5-flash", model),
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
		sess.mu.Unlock()

		if err := engine.StepBlocks(ctx, blocks); err != nil {
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

func buildUserBlocks(message string, images []ImageAttachment) ([]llm.ContentBlock, error) {
	var blocks []llm.ContentBlock
	if strings.TrimSpace(message) != "" {
		blocks = append(blocks, llm.ContentBlock{Type: llm.BlockText, Text: message})
	}
	for _, img := range images {
		data, err := base64.StdEncoding.DecodeString(img.DataB64)
		if err != nil {
			return nil, fmt.Errorf("decode image %s: %w", img.Name, err)
		}
		if len(data) == 0 {
			continue
		}
		blocks = append(blocks, llm.ImageBlock(img.MimeType, data))
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("empty message")
	}
	return blocks, nil
}
