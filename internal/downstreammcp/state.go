package downstreammcp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	TransportCommand = "command"
	TransportSSE     = "sse"
	DefaultTimeoutMS = 30000
	DefaultMaxBytes  = 20000
)

type State struct {
	Servers []Server `yaml:"servers" json:"servers"`
}

type Server struct {
	Name        string   `yaml:"name" json:"name"`
	Transport   string   `yaml:"transport" json:"transport"`
	Command     string   `yaml:"command,omitempty" json:"command,omitempty"`
	Args        []string `yaml:"args,omitempty" json:"args,omitempty"`
	URL         string   `yaml:"url,omitempty" json:"url,omitempty"`
	TimeoutMS   int      `yaml:"timeout_ms,omitempty" json:"timeout_ms,omitempty"`
	MaxBytes    int      `yaml:"max_bytes,omitempty" json:"max_bytes,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
}

func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, nil
		}
		return State{}, err
	}
	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func Save(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (s State) Get(name string) (Server, bool) {
	for _, server := range s.Servers {
		if server.Name == name {
			return server.withDefaults(), true
		}
	}
	return Server{}, false
}

func (s *State) Upsert(server Server) {
	server = server.withDefaults()
	for i := range s.Servers {
		if s.Servers[i].Name == server.Name {
			s.Servers[i] = server
			return
		}
	}
	s.Servers = append(s.Servers, server)
}

func (s State) PublicServers() []Server {
	out := make([]Server, 0, len(s.Servers))
	for _, server := range s.Servers {
		out = append(out, server.withDefaults())
	}
	return out
}

func (s Server) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("server name is required")
	}
	switch s.Transport {
	case TransportCommand:
		if strings.TrimSpace(s.Command) == "" {
			return errors.New("command transport requires command")
		}
	case TransportSSE:
		if strings.TrimSpace(s.URL) == "" {
			return errors.New("sse transport requires url")
		}
	default:
		return errors.New("transport must be command or sse")
	}
	return nil
}

func (s Server) withDefaults() Server {
	if s.TimeoutMS <= 0 {
		s.TimeoutMS = DefaultTimeoutMS
	}
	if s.MaxBytes <= 0 {
		s.MaxBytes = DefaultMaxBytes
	}
	return s
}
