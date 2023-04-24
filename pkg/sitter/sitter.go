package sitter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/sleexyz/dev-world/pkg/workspace"
)

const (
	WORKSPACE_PATH_COOKIE = "workspace-path"
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

func (s *Sitter) GetWorkspaces() []string {
	s.workspaceMapMu.Lock()
	defer s.workspaceMapMu.Unlock()
	ws := make([]string, 0, len(s.workspaceMap))
	for k := range s.workspaceMap {
		ws = append(ws, s.workspaceMap[k].Path)
	}
	return ws
}

func (s *Sitter) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	// log.Printf("Proxying request: %s\n", r.URL.Path)
	ws, err := s.GetWorkspaceForRequest(w, r)
	if err != nil {
		return
	}

	err = ws.ReverseProxy(w, r)
	if err != nil {
		var netOpError *net.OpError
		if ok := errors.As(err, &netOpError); ok {
			if netOpError.Op == "dial" {
				log.Printf("Restarting workspace %s\n", ws.Path)
				s.deleteWorkspace(ws)
				err = ws.ReverseProxy(w, r)
				if err != nil {
					log.Printf("Error proxying request: %s\n", err)
					http.Error(w, "Error proxying request", http.StatusInternalServerError)
					return
				}
			}
		} else {
			log.Printf("Error proxying request: %s\n", err)
			http.Error(w, "Error proxying request", http.StatusInternalServerError)
			return
		}
	}
}

func (s *Sitter) GetWorkspaceForRequest(w http.ResponseWriter, r *http.Request) (*workspace.Workspace, error) {
	var path string
	// Get the folder from either the query string or the cookie
	cookie, err := r.Cookie(WORKSPACE_PATH_COOKIE)
	path = r.URL.Query().Get("folder")
	if cookie != nil && path == "" {
		path = cookie.Value
	} else if path == "" {
		http.Error(w, "Could not determine folder", http.StatusBadRequest)
		return nil, err
	}

	ws, err := s.GetWorkspace(r.Context(), path)
	if err != nil {
		ws = workspace.CreateWorkspace(r.Context(), path)
		s.addWorkspace(ws)
	}

	if cookie == nil {
		cookie := http.Cookie{Name: WORKSPACE_PATH_COOKIE, Value: ws.Path, Path: "/"}
		http.SetCookie(w, &cookie)
	}

	return ws, nil
}

// getWorkspace returns a workspace for the given path hash. If the workspace
// doesn't exist, it will be created and added to the sitter.
func (s *Sitter) GetWorkspace(ctx context.Context, path string) (*workspace.Workspace, error) {
	s.workspaceMapMu.Lock()
	defer s.workspaceMapMu.Unlock()

	if workspace, ok := s.workspaceMap[path]; ok {
		return workspace, nil
	}

	return nil, fmt.Errorf("workspace not found")
}

func (sitter *Sitter) addWorkspace(workspace *workspace.Workspace) {
	sitter.workspaceMapMu.Lock()
	defer sitter.workspaceMapMu.Unlock()
	sitter.workspaceMap[workspace.Path] = workspace
	log.Printf("Added workspace: %s \n", workspace.Path)
}

func (sitter *Sitter) deleteWorkspace(workspace *workspace.Workspace) {
	sitter.workspaceMapMu.Lock()
	defer sitter.workspaceMapMu.Unlock()
	workspace.Close()
	delete(sitter.workspaceMap, workspace.Path)
	log.Printf("Deleted workspace: %s \n", workspace.Path)
}

func (s *Sitter) DeleteWorkspace(path string) error {
	ws, err := s.GetWorkspace(context.Background(), path)
	if err != nil {
		return err
	}
	s.deleteWorkspace(ws)
	return nil
}
