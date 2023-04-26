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

	// Attempt reconnection to existing workspaces, and then re-save the sitter state
	for _, ws := range sitterState.Workspaces {
		w, err := sitter.reconnectToWorkspace(context.Background(), ws)
		// If we fail to reconnect to a workspace, remove the stale socket
		if err != nil {
			codeServerSocketPath := workspace.GetCodeServerSocketPath(ws.Path)
			err := os.Remove(codeServerSocketPath)
			if err != nil {
				log.Fatalf("Failed to remove existing socket: %v", err)
			}
			log.Printf("Removed stale socket at %s\n", ws.Path)
		}
		sitter.addWorkspace(w)
	}
	sitter.SaveSitter()

	// Clean up any stale sockets. This is necessary so that we can create fresh sockets
	// without overlap.
	dirEntries, err := os.ReadDir(os.TempDir())
	if err != nil {
		log.Fatalf("Failed to read temp dir: %v", err)
	}
	for _, dirEntry := range dirEntries {
		if err != nil || dirEntry.IsDir() {
			continue
		}
		path, err := workspace.GetFolderFromSocketPath(dirEntry.Name())
		if err != nil {
			continue
		}
		if sitter.workspaceMap[path] == nil {
			log.Printf("Removing stale socket at %s\n", dirEntry.Name())
			socketPath := os.TempDir() + dirEntry.Name()
			err := os.Remove(socketPath)
			if err != nil {
				log.Fatalf("Failed to remove existing socket: %v", err)
			}
		}
	}

	return sitter
}

// Saves the sitter state to disk
func (s *Sitter) SaveSitter() {
	var SitterState SitterState
	SitterState.Workspaces = make(map[string]*WorkspaceState)
	for _, ws := range s.workspaceMap {
		SitterState.Workspaces[ws.Path] = &WorkspaceState{
			Path:   ws.Path,
			Socket: ws.Socket,
			Pid:    ws.Process.Pid,
		}
	}
	SaveSitterState(&SitterState)
}

func (s *Sitter) reconnectToWorkspace(ctx context.Context, ws *WorkspaceState) (*workspace.Workspace, error) {
	codeServerSocketPath := workspace.GetCodeServerSocketPath(ws.Path)

	_, err := os.Stat(codeServerSocketPath)
	if err != nil {
		return nil, err
	}

	process, err := getCodeServerProcess(ctx, codeServerSocketPath)
	if err != nil {
		return nil, err
	}

	// If the socket already exists, try to reconnect to it
	w := &workspace.Workspace{
		Path:    ws.Path,
		Socket:  codeServerSocketPath,
		Process: process,
	}
	err = w.WaitForSocket(ctx)
	if err != nil {
		return nil, err
	}
	log.Printf("Reconnecting to existing socket for %s\n", ws.Path)
	return w, nil
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
