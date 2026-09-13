package downstreammcp

import (
	"context"
	"errors"
	"strings"
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
	snapshot, err := NewFileStore(path).Read(context.Background())
	if err != nil {
		return State{}, err
	}
	return snapshot.State, nil
}

func Save(path string, state State) error {
	return writeStateAtomically(path, state)
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

func (s *State) Remove(name string) (Server, bool) {
	for i := range s.Servers {
		if s.Servers[i].Name == name {
			removed := s.Servers[i].withDefaults()
			s.Servers = append(s.Servers[:i], s.Servers[i+1:]...)
			return removed, true
		}
	}
	return Server{}, false
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

func (s Server) equals(other Server) bool {
	left := s.withDefaults()
	right := other.withDefaults()
	if left.Name != right.Name ||
		left.Transport != right.Transport ||
		left.Command != right.Command ||
		left.URL != right.URL ||
		left.TimeoutMS != right.TimeoutMS ||
		left.MaxBytes != right.MaxBytes ||
		left.Description != right.Description ||
		len(left.Args) != len(right.Args) {
		return false
	}
	for i := range left.Args {
		if left.Args[i] != right.Args[i] {
			return false
		}
	}
	return true
}
