// Package a2a implements a minimal, interoperable subset of the Agent-to-Agent
// (A2A) protocol so AgentCTL agents can expose themselves as network services
// and call one another. It is transport-agnostic at the core: the server is
// given an Executor and never imports the engine, which keeps it testable.
//
// The wire format follows the A2A Agent Card + JSON-RPC `message/send`
// conventions closely enough to interoperate with other A2A tooling, while
// staying intentionally small (synchronous, text parts, no push
// notifications) for the first version.
package a2a

import "github.com/subzone/Agentctl/internal/config"

// AgentCard is the public description an A2A agent serves at
// /.well-known/agent.json. Field names match the A2A spec.
type AgentCard struct {
	Name               string       `json:"name"`
	Description        string       `json:"description"`
	URL                string       `json:"url"`
	Version            string       `json:"version"`
	Capabilities       Capabilities `json:"capabilities"`
	DefaultInputModes  []string     `json:"defaultInputModes"`
	DefaultOutputModes []string     `json:"defaultOutputModes"`
	Skills             []Skill      `json:"skills"`
}

// Capabilities advertises optional protocol features. This implementation is
// synchronous request/response, so streaming and push are false.
type Capabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"pushNotifications"`
}

// Skill is a capability the agent advertises. We map one agent to one primary
// skill derived from its description.
type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

// CardFromAgent builds an AgentCard from a parsed agent spec. baseURL is the
// externally reachable root (e.g. "http://localhost:8080"); version is the
// build version string. Both may be empty and are filled with sane defaults.
func CardFromAgent(spec *config.AgentSpec, baseURL, version string) AgentCard {
	if version == "" {
		version = "0.0.0"
	}
	desc := spec.Description
	if desc == "" {
		desc = "AgentCTL agent " + spec.Name
	}
	tags := make([]string, 0, len(spec.Tools))
	tags = append(tags, spec.Tools...)
	return AgentCard{
		Name:               spec.Name,
		Description:        desc,
		URL:                baseURL,
		Version:            version,
		Capabilities:       Capabilities{Streaming: false, PushNotifications: false},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []Skill{{
			ID:          spec.Name,
			Name:        spec.Name,
			Description: desc,
			Tags:        tags,
		}},
	}
}
