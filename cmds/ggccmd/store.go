package ggccmd

import (
	"os"
	"path/filepath"

	"github.com/pubgo/fastgit/configs"
	"gopkg.in/yaml.v3"
)

type StateStore struct {
	path string
}

type GGCState struct {
	Workflows [][]string          `yaml:"workflows,omitempty"`
	Aliases   map[string]AliasDef `yaml:"aliases,omitempty"`
}

func NewStateStore() *StateStore {
	return &StateStore{path: filepath.Join(filepath.Dir(configs.GetConfigPath()), "ggc.yaml")}
}

func (s *StateStore) Path() string {
	return s.path
}

func (s *StateStore) Load() (*GGCState, error) {
	state := defaultState()

	if _, err := os.Stat(s.path); err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return nil, err
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return state, nil
	}

	if err := yaml.Unmarshal(data, state); err != nil {
		return nil, err
	}

	state.normalize()
	return state, nil
}

func (s *StateStore) Save(state *GGCState) error {
	state.normalize()

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

func defaultState() *GGCState {
	return &GGCState{
		Workflows: [][]string{{}},
		Aliases:   map[string]AliasDef{},
	}
}

func (s *GGCState) normalize() {
	if s.Aliases == nil {
		s.Aliases = map[string]AliasDef{}
	}
	if len(s.Workflows) == 0 {
		s.Workflows = [][]string{{}}
		return
	}
	for i := range s.Workflows {
		if s.Workflows[i] == nil {
			s.Workflows[i] = []string{}
		}
	}
}
