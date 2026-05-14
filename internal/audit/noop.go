package audit

import (
	"context"

	"github.com/subzone/Agentctl/internal/ports"
)

type noopSink struct{}

func NewNoopSink() ports.AuditSink { return &noopSink{} }

func (n *noopSink) Emit(_ context.Context, _ ports.AuditEvent) error { return nil }
func (n *noopSink) Flush(_ context.Context) error                    { return nil }
func (n *noopSink) Close() error                                     { return nil }
