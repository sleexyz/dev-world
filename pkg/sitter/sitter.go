package sitter

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

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

func InitializeSitter() *Sitter {
	sitter := CreateNewSitter()
	pattern := regexp.MustCompile(`code-server-(.+)\.sock`)
	files, err := ioutil.ReadDir("/tmp")
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		if file.IsDir() || (file.Mode()&os.ModeSocket) == 0 {
			continue
		}
		if matches := pattern.FindStringSubmatch(file.Name()); len(matches) > 0 {
			key := matches[1]
			folder, err := base64.StdEncoding.DecodeString(key)
			if err != nil {
				log.Printf("Invalid workspace key: %s", key)
				continue
			}
			sitter.createWorkspace(context.Background(), string(folder))
		}
	}
	return sitter
}

func (s *Sitter) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Proxying request: %s\n", r.URL.Path)
	folder := r.URL.Query().Get("folder")
	if folder == "" {
		cookie, err := r.Cookie("workspace-key")
		if err != nil {
			http.Error(w, "Missing folder query parameter", http.StatusBadRequest)
			return
		}
		key := cookie.Value
		folder, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			http.Error(w, "Invalid workspace key", http.StatusBadRequest)
			return
		}
		workspace := s.GetWorkspace(r.Context(), string(folder))
		workspace.ReverseProxy(w, r)
		return
	}
	workspace := s.GetWorkspace(r.Context(), folder)
	cookie := http.Cookie{Name: "workspace-key", Value: workspace.Key, Path: "/"}
	http.SetCookie(w, &cookie)
	workspace.ReverseProxy(w, r)
}

// getWorkspace returns a workspace for the given path hash. If the workspace
// doesn't exist, it will be created.
func (s *Sitter) GetWorkspace(ctx context.Context, folder string) *workspace.Workspace {
	s.workspaceMapMu.Lock()
	defer s.workspaceMapMu.Unlock()

	if workspace, ok := s.workspaceMap[folder]; ok {
		return workspace
	}
	return s.createWorkspace(ctx, folder)
}

// use pgrep to find the process that contains the socket as a command line argument
func getMatchingProcess(ctx context.Context, socketPath string) (*os.Process, error) {
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

func (s *Sitter) createWorkspace(ctx context.Context, folder string) *workspace.Workspace {
	key := base64.StdEncoding.EncodeToString([]byte(folder))

	codeServerSocketPath := fmt.Sprintf("/tmp/code-server-%s.sock", key)

	// If the socket already exists, try to reconnect to it
	if _, err := os.Stat(codeServerSocketPath); err == nil {
		if process, err := getMatchingProcess(ctx, codeServerSocketPath); err == nil {
			log.Printf("Reconnecting to existing socket at %s\n", folder)
			workspace := &workspace.Workspace{
				Key:                  key,
				Folder:               folder,
				CodeServerSocketPath: codeServerSocketPath,
				Process:              process,
			}
			s.workspaceMap[folder] = workspace
			return workspace
		}
		// If the socket exists but the process doesn't, remove the socket
		if err := os.Remove(codeServerSocketPath); err != nil {
			log.Fatalf("Failed to remove existing socket: %v", err)
		}
	}

	_, err := os.Create(codeServerSocketPath)
	if err != nil {
		log.Fatalln("Error creating socket:", err)
	}

	// Start a new child process for the folder
	cmd := exec.Command("code-server", "--socket", codeServerSocketPath, folder)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Prevent child process from being killed when parent process exits
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start child process: %v", err)
	}

	workspace := &workspace.Workspace{
		Key:                  key,
		Folder:               folder,
		CodeServerSocketPath: codeServerSocketPath,
		Process:              cmd.Process,
	}

	// Add the workspace to the map
	s.workspaceMap[folder] = workspace

	// Wait for the child process to exit and remove the workspace from the map
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("Child process terminated: %v", err)
			delete(s.workspaceMap, folder)
		}
	}()

	err = workspace.WaitForSocket(ctx)
	if err != nil {
		log.Printf("Failed health check for child process: %v", err)
	}

	return s.workspaceMap[folder]
}
