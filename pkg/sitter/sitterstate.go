package sitter

import (
	"encoding/json"
	"os"

	"github.com/sleexyz/dev-world/pkg/workspace"
)

var sitterStatePath = os.Getenv("TMPDIR") + "/dev-world-state.json"

type SitterState struct {
	WorkspaceMap map[string]*workspace.Workspace `json:"workspaceMap"`
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
