package sitter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

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

func (s *Sitter) GetWorkspaceForFile(file string) *workspace.Workspace {
	s.workspaceMapMu.Lock()
	defer s.workspaceMapMu.Unlock()
	for _, ws := range s.workspaceMap {
		// Return workspace if the file is a subpath of the workspace path
		if len(file) >= len(ws.Path) && file[:len(ws.Path)] == ws.Path {
			return ws
		}
	}
	return nil
}

func (s *Sitter) GetWorkspacePaths() []string {
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
	ws, err := s.GetOrCreateWorkspace(w, r)
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

func (s *Sitter) GetOrCreateWorkspace(w http.ResponseWriter, r *http.Request) (*workspace.Workspace, error) {
	s.workspaceMapMu.Lock()
	defer s.workspaceMapMu.Unlock()

	// Get the path from the query string, or the cookie.
	// The query string takes precedence.
	cookie, _ := r.Cookie(WORKSPACE_PATH_COOKIE)
	path := r.URL.Query().Get("folder")
	if path == "" {
		if cookie != nil {
			path = cookie.Value
		} else {
			http.Error(w, "Could not determine folder", http.StatusBadRequest)
			return nil, fmt.Errorf("could not determine folder")
		}
	} else {
		http.SetCookie(w, &http.Cookie{
			Name:     WORKSPACE_PATH_COOKIE,
			Value:    path,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
	}

	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(5*time.Second))
	defer cancel()

	ws, err := s.GetWorkspace(ctx, path)
	if err != nil {
		ws = workspace.CreateWorkspace(ctx, path)
		go func() {
			ws.VscodeSocket = <-workspace.WaitForVscodeSocket(ws.Process.Pid)
			s.SaveSitter()
		}()

		s.addWorkspace(ws)
		s.workspaceMap[ws.Path] = ws
		s.SaveSitter()
		log.Printf("Added workspace: %s \n", ws.Path)
	}

	return ws, nil
}

func (s *Sitter) GetWorkspace(ctx context.Context, path string) (*workspace.Workspace, error) {
	if workspace, ok := s.workspaceMap[path]; ok {
		return workspace, nil
	}

	return nil, fmt.Errorf("workspace not found")
}

func (sitter *Sitter) addWorkspace(workspace *workspace.Workspace) {
}

func (sitter *Sitter) deleteWorkspace(workspace *workspace.Workspace) {
	workspace.Close()
	delete(sitter.workspaceMap, workspace.Path)
	sitter.SaveSitter()

	log.Printf("Deleted workspace: %s \n", workspace.Path)
}

func (s *Sitter) DeleteWorkspace(path string) error {
	s.workspaceMapMu.Lock()
	defer s.workspaceMapMu.Unlock()
	ws, err := s.GetWorkspace(context.Background(), path)
	if err != nil {
		return err
	}
	s.deleteWorkspace(ws)
	return nil
}
