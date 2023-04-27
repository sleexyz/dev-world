package sitter

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/sleexyz/dev-world/pkg/workspace"
)

var (
	skipReconnectionFlag = flag.Bool("skip-reconnection", false, "Skip reconnection to existing workspaces")
)

// Loads the sitter state from disk and reconnects to any existing workspaces
func LoadSitter() *Sitter {
	sitter := CreateNewSitter()
	var sitterState *SitterState
	if *skipReconnectionFlag {
		log.Println("Skipping reconnection to existing workspaces")
		sitterState = &SitterState{}
	} else {
		sitterState = LoadSitterState()
	}

	// Attempt reconnection to existing workspaces, and then re-save the sitter state
	for _, ws := range sitterState.Workspaces {
		w, err := sitter.restoreWorkspace(context.Background(), ws)
		if err != nil {
			log.Printf("Failed to reconnect to workspace at %s: %v\n", ws.Path, err)
			continue
		}
		log.Printf("Reconnected to workspace at %s\n", ws.Path)
		sitter.workspaceMap[ws.Path] = w
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
	var ss SitterState
	ss.Workspaces = make(map[string]*WorkspaceState)
	for _, ws := range s.workspaceMap {
		ss.Workspaces[ws.Path] = &WorkspaceState{
			Path:         ws.Path,
			Socket:       ws.Socket,
			VscodeSocket: ws.VscodeSocket,
			Pid:          ws.Process.Pid,
		}
	}
	SaveSitterState(&ss)
}

func (s *Sitter) restoreWorkspace(ctx context.Context, ws *WorkspaceState) (*workspace.Workspace, error) {
	// Check if the socket exists and is ready
	_, err := os.Stat(ws.Socket)
	if err != nil {
		return nil, err
	}
	if err = workspace.WaitForSocket(ctx, ws.Socket); err != nil {
		return nil, err
	}

	// Check if the vscode socket exists and is ready
	_, err = os.Stat(ws.VscodeSocket)
	if err != nil {
		return nil, err
	}
	if err = workspace.WaitForSocket(ctx, ws.VscodeSocket); err != nil {
		return nil, err
	}

	// Check if the process exists
	process, err := os.FindProcess(ws.Pid)
	if err != nil {
		return nil, err
	}

	w := &workspace.Workspace{
		Path:         ws.Path,
		Socket:       ws.Socket,
		VscodeSocket: ws.VscodeSocket,
		Process:      process,
	}

	return w, nil
}
