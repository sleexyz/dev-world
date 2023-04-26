package sitter

import (
	"encoding/json"
	"os"
)

var sitterStatePath = os.Getenv("TMPDIR") + "/dev-world-state.json"

type SitterState struct {
	Workspaces map[string]*WorkspaceState `json:"workspaces"`
}

type WorkspaceState struct {
	Path   string `json:"path"`
	Socket string `json:"socket"`
	Pid    int    `json:"pid"`
}

func LoadSitterState() *SitterState {
	var SitterState SitterState

	if _, err := os.Stat(sitterStatePath); err == nil {
		f, err := os.Open(sitterStatePath)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := json.NewDecoder(f).Decode(&SitterState); err != nil {
			panic(err)
		}
	}

	return &SitterState
}

func SaveSitterState(sitterState *SitterState) {
	f, err := os.Create(sitterStatePath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(&sitterState); err != nil {
		panic(err)
	}
}
