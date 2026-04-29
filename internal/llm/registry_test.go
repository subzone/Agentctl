package llm

import (
	"context"
	"errors"
	"testing"
)

type stubProvider struct{}

func (stubProvider) Stream(context.Context, Request) (<-chan Event, error) {
	ch := make(chan Event)
	close(ch)
	return ch, nil
}

func TestResolve(t *testing.T) {
	Register("stub-resolve", func() (Provider, error) { return stubProvider{}, nil })

	p, model, err := Resolve("stub-resolve/some-model-v1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "some-model-v1" {
		t.Errorf("model = %q, want some-model-v1", model)
	}
	if p == nil {
		t.Error("nil provider")
	}
}

func TestResolveUnknownProvider(t *testing.T) {
	_, _, err := Resolve("nope-xyz/whatever")
	if !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("got %v, want ErrUnknownProvider", err)
	}
}

func TestResolveBadFormat(t *testing.T) {
	for _, m := range []string{"", "noProvider", "/no-name", "name/"} {
		if _, _, err := Resolve(m); err == nil {
			t.Errorf("Resolve(%q): expected error", m)
		}
	}
}

func TestResolveFactoryError(t *testing.T) {
	want := errors.New("boom")
	Register("stub-fail", func() (Provider, error) { return nil, want })
	_, _, err := Resolve("stub-fail/x")
	if !errors.Is(err, want) {
		t.Errorf("got %v, want wraps %v", err, want)
	}
}
