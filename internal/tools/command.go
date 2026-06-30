package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CommandTool is a user-defined tool backed by an external command (a tool MD
// with runtime: shell). The model's JSON input is written to the command's
// stdin; stdout (and stderr on failure) is returned to the model. This lets
// users add capabilities — scripts, CLIs, HTTP shims — without touching Go.
type CommandTool struct {
	name        string
	description string
	command     []string
	env         map[string]string
	schema      json.RawMessage
	timeout     time.Duration
	maxOutput   int
}

// NewCommandTool builds a command-backed tool. command[0] is the executable
// and the rest are fixed args. schema is the JSON-Schema advertised to the
// model; if nil, an empty object schema is used. timeout <= 0 defaults to 60s.
func NewCommandTool(name, description string, command []string, env map[string]string, schema json.RawMessage, timeout time.Duration) *CommandTool {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if len(schema) == 0 {
		schema = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return &CommandTool{
		name:        name,
		description: description,
		command:     command,
		env:         env,
		schema:      schema,
		timeout:     timeout,
		maxOutput:   32 * 1024,
	}
}

func (c *CommandTool) Name() string { return c.name }

func (c *CommandTool) Description() string { return c.description }

func (c *CommandTool) InputSchema() json.RawMessage { return c.schema }

func (c *CommandTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	if len(c.command) == 0 {
		return "", fmt.Errorf("tool %q has no command", c.name)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// #nosec G204 -- the command is operator-defined in a tool MD file, not
	// model-controlled; the model only supplies stdin input.
	cmd := exec.CommandContext(ctx, c.command[0], c.command[1:]...)

	cmd.Env = os.Environ()
	for k, v := range c.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if len(input) > 0 && string(input) != "null" {
		cmd.Stdin = bytes.NewReader(input)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	text := out.String()
	if len(text) > c.maxOutput {
		text = text[:c.maxOutput] + "\n[output truncated]"
	}

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("tool %q timed out after %s", c.name, c.timeout)
	}
	if err != nil {
		// Surface the exit error inline so the model can react, consistent
		// with the shell tool's convention of returning output + nil.
		if _, ok := err.(*exec.ExitError); ok {
			return strings.TrimRight(text, "\n") + fmt.Sprintf("\n[exit error: %v]", err), nil
		}
		return "", fmt.Errorf("run %q: %w", c.name, err)
	}
	return text, nil
}
