package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/a2a"
	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/tools"
)

func newA2ACmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "a2a",
		Short: "Expose and call agents over the Agent-to-Agent (A2A) protocol",
		Long: "Serve an agent as an A2A network service, or call a remote A2A agent.\n\n" +
			"Examples:\n" +
			"  m a2a serve coder --addr :8080   # expose the coder agent\n" +
			"  m a2a card http://localhost:8080 # fetch a remote agent's card\n" +
			"  m a2a call http://localhost:8080 \"review main.go\"",
	}
	cmd.AddCommand(newA2AServeCmd())
	cmd.AddCommand(newA2ACallCmd())
	cmd.AddCommand(newA2ACardCmd())
	return cmd
}

func newA2AServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve <agent>",
		Short: "Serve an agent as an A2A endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runA2AServe(cmd, args[0], addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "Address to listen on")
	return cmd
}

func newA2ACallCmd() *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "call <url> <message...>",
		Short: "Send a task to a remote A2A agent",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}
			msg, err := readTask(cmd, args[1:])
			if err != nil {
				return err
			}
			reply, err := a2a.NewClient(args[0]).Send(ctx, msg)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), reply)
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Request timeout")
	return cmd
}

func newA2ACardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "card <url>",
		Short: "Fetch and print a remote agent's A2A card",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			card, err := a2a.NewClient(args[0]).Card(ctx)
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(card, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
}

func runA2AServe(cmd *cobra.Command, agentRef, addr string) error {
	path := resolveAgentPath(agentRef)
	doc, err := config.ParseFile(path)
	if err != nil {
		return err
	}
	agent, ok := doc.Spec.(*config.AgentSpec)
	if !ok {
		return fmt.Errorf("%s: not an agent (type=%s)", agentRef, doc.Meta().Type)
	}
	if issues := config.Validate(doc); len(issues) > 0 {
		for _, iss := range issues {
			fmt.Fprintln(cmd.ErrOrStderr(), iss.Error())
		}
		return fmt.Errorf("agent failed validation")
	}

	provider, model, err := llm.Resolve(agent.Model)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	stderr := cmd.ErrOrStderr()
	docs := loadCompanionDocs(path)

	// Auto-approve tool calls (no interactive prompt on a server) but still
	// refuse obviously dangerous shell commands.
	autoConfirm := func(_ context.Context, name string, input json.RawMessage) (bool, error) {
		if pattern := isDangerousCommand(name, input); pattern != "" {
			fmt.Fprintf(stderr, "⚠️  blocked dangerous %q in %s\n", pattern, name)
			return false, nil
		}
		return true, nil
	}

	hubSpawner := &spawner{
		docs:       docs,
		out:        io.Discard,
		status:     stderr,
		confirm:    func(_ context.Context, _ string) (bool, error) { return true, nil },
		undo:       &tools.UndoStack{},
		spawnDepth: 1,
	}
	rt, err := buildAgentRuntime(ctx, agent, docs, hubSpawner, stderr)
	if err != nil {
		return err
	}
	defer rt.close()

	maxTokens := defaultMaxTokens
	if agent.MaxTokens != nil {
		maxTokens = *agent.MaxTokens
	}

	// Requests are serialized: the shared tool runtime (MCP managers, undo
	// stack) is not designed for concurrent use, and per-request sessions keep
	// conversations isolated.
	var mu sync.Mutex
	exec := func(reqCtx context.Context, task string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		var buf bytes.Buffer
		sess := engine.NewSession(engine.Config{
			Provider:       provider,
			Model:          model,
			System:         systemWithCwd(doc.Body),
			Tools:          rt.registry,
			Temperature:    agent.Temperature,
			MaxTokens:      maxTokens,
			FallbackModels: agent.FallbackModels,
			ResolveModel:   llm.Resolve,
			Routing:        agent.Routing,
			ToolConfirm:    autoConfirm,
			Out:            &buf,
			Status:         io.Discard,
		})
		err := sess.Step(reqCtx, task)
		return buf.String(), err
	}

	baseURL := "http://localhost" + addr
	card := a2a.CardFromAgent(agent, baseURL, Version)
	srv := &http.Server{
		Addr:              addr,
		Handler:           a2a.NewServer(card, exec).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = srv.Shutdown(shutCtx)
	}()

	fmt.Fprintf(stderr, "A2A: serving agent %q on %s\n", agent.Name, addr)
	fmt.Fprintf(stderr, "  card:  GET  %s/.well-known/agent.json\n", baseURL)
	fmt.Fprintf(stderr, "  send:  POST %s/  (JSON-RPC message/send)\n", baseURL)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
