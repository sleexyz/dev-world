package sitter

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sleexyz/dev-world/pkg/workspace"
)

// Loads the sitter state from disk and reconnects to any existing workspaces
func LoadSitter() *Sitter {
	sitter := CreateNewSitter()
	sitterState := LoadSitterState()

	// Attempt reconnection to existing workspaces, and then re-save the sitter state.
	for _, ws := range sitterState.WorkspaceMap {
		ws, err := sitter.reconnectToWorkspace(context.Background(), ws.Path)
		if err != nil {
			cleanupDeadWorkspace(ws.Path)
		}
		sitter.addWorkspace(ws)
	}
	sitter.SaveSitter()

	return sitter
}

// Saves the sitter state to disk
func (s *Sitter) SaveSitter() {
	var SitterState SitterState
	SitterState.WorkspaceMap = make(map[string]*workspace.Workspace)
	for _, ws := range s.workspaceMap {
		SitterState.WorkspaceMap[ws.Path] = ws
	}
	SaveSitterState(&SitterState)
}

func cleanupDeadWorkspace(path string) {
	codeServerSocketPath := workspace.GetCodeServerSocketPath(path)
	if err := os.Remove(codeServerSocketPath); err != nil {
		log.Fatalf("Failed to remove existing socket: %v", err)
	}
	log.Printf("Removed stale socket at %s\n", path)
}

func (s *Sitter) reconnectToWorkspace(ctx context.Context, path string) (*workspace.Workspace, error) {
	codeServerSocketPath := workspace.GetCodeServerSocketPath(path)

	_, err := os.Stat(codeServerSocketPath)
	if err != nil {
		return nil, err
	}

	process, err := getCodeServerProcess(ctx, codeServerSocketPath)
	if err != nil {
		return nil, err
	}

	// If the socket already exists, try to reconnect to it
	workspace := &workspace.Workspace{
		Path:    path,
		Socket:  codeServerSocketPath,
		Process: process,
	}
	err = workspace.WaitForSocket(ctx)
	if err != nil {
		return nil, err
	}
	log.Printf("Reconnecting to existing socket for %s\n", path)
	return workspace, nil
}

func getCodeServerProcess(ctx context.Context, socketPath string) (*os.Process, error) {
	cmd := exec.CommandContext(ctx, "pgrep", "-f", socketPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	outs := strings.Split(string(out), "\n")
	for _, out := range outs {
		pid, err := strconv.Atoi(string(out))
		if err != nil {
			continue
		}
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		return process, nil
	}
	return nil, fmt.Errorf("no matching process found")
}
