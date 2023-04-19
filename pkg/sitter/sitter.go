package sitter

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/sleexyz/dev-world/pkg/workspace"
)

type Sitter struct {
	workspaceMap   map[string]*workspace.Workspace
	workspaceMapMu sync.Mutex
}

func CreateNewSitter() *Sitter {
	return &Sitter{
		workspaceMap:   make(map[string]*workspace.Workspace),
		workspaceMapMu: sync.Mutex{},
	}
}

func (s *Sitter) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	// log.Printf("Proxying request: %s\n", r.URL.Path)
	ws, err := s.GetWorkspaceForRequest(w, r)
	if err != nil {
		return
	}
	ws.ReverseProxy(w, r)
}

func (s *Sitter) GetWorkspaceForRequest(w http.ResponseWriter, r *http.Request) (*workspace.Workspace, error) {
	var folder string
	var key string
	// Get the folder from either the query string or the cookie
	folder = r.URL.Query().Get("folder")
	cookie, err := r.Cookie("workspace-key")
	if folder == "" && cookie == nil {
		http.Error(w, "Could not determine folder", http.StatusBadRequest)
		return nil, err
	} else if folder == "" {
		key = cookie.Value
		rawFolder, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			http.Error(w, "Invalid workspace key", http.StatusBadRequest)
			return nil, err
		}
		folder = string(rawFolder)
	} else { // folder != ""
		key = workspace.CreateKeyFromFolder(folder)
	}

	ws, err := s.GetWorkspace(r.Context(), key)
	if err != nil {
		ws = workspace.CreateWorkspace(r.Context(), folder)
		s.addWorkspace(ws)
	}

	if cookie == nil {
		cookie := http.Cookie{Name: "workspace-key", Value: ws.Key, Path: "/"}
		http.SetCookie(w, &cookie)
	}

	return ws, nil
}

// getWorkspace returns a workspace for the given path hash. If the workspace
// doesn't exist, it will be created and added to the sitter.
func (s *Sitter) GetWorkspace(ctx context.Context, key string) (*workspace.Workspace, error) {
	s.workspaceMapMu.Lock()
	defer s.workspaceMapMu.Unlock()

	if workspace, ok := s.workspaceMap[key]; ok {
		return workspace, nil
	}

	return nil, fmt.Errorf("workspace not found")
}

func (sitter *Sitter) addWorkspace(workspace *workspace.Workspace) {
	sitter.workspaceMapMu.Lock()
	defer sitter.workspaceMapMu.Unlock()
	sitter.workspaceMap[workspace.Key] = workspace
	log.Printf("Added workspace: %s \n", workspace.Key)
}
