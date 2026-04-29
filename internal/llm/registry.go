package llm

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Factory constructs a Provider on demand. It runs once per Resolve call so
// providers can read env vars / secrets at construction time and surface
// configuration errors clearly.
type Factory func() (Provider, error)

var (
	regMu     sync.RWMutex
	factories = map[string]Factory{}
)

// ErrUnknownProvider is returned when a model string references a provider
// that has not been registered.
var ErrUnknownProvider = errors.New("unknown llm provider")

// Register makes a provider available under name. Typically called from a
// provider package's init().
func Register(name string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	factories[name] = f
}

// Resolve splits a "provider/model" string, looks up the provider factory,
// and returns the constructed provider plus the bare model id.
func Resolve(model string) (Provider, string, error) {
	name, bare, ok := strings.Cut(model, "/")
	if !ok || name == "" || bare == "" {
		return nil, "", fmt.Errorf("model %q must be in form provider/model", model)
	}
	regMu.RLock()
	f, found := factories[name]
	regMu.RUnlock()
	if !found {
		return nil, "", fmt.Errorf("%w: %q", ErrUnknownProvider, name)
	}
	p, err := f()
	if err != nil {
		return nil, "", fmt.Errorf("init provider %s: %w", name, err)
	}
	return p, bare, nil
}
