package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Workspace struct {
	pathHash         string
	vscodeSocketPath string
	process          *os.Process
}

var (
	// workspaceMap maps path hashes to workspaces.
	workspaceMap   = make(map[string]*Workspace)
	workspaceMapMu sync.Mutex
)

func (workspace *Workspace) reverseProxy(w http.ResponseWriter, r *http.Request) {
	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", workspace.vscodeSocketPath)
		},
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = r.Host
		},
		Transport: transport,
	}
	proxy.ServeHTTP(w, r)
}

func (workspace *Workspace) waitForSocket(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	backoff := time.Millisecond * 100
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := net.Dial("unix", workspace.vscodeSocketPath)
			if err == nil {
				conn.Close()
				return nil
			}
			fmt.Println("Server is not alive, waiting...")
			time.Sleep(backoff)
			backoff *= 2
		}
	}
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folder")
	if folder == "" {
		cookie, err := r.Cookie("pathHash")
		if err != nil {
			http.Error(w, "Missing folder query parameter", http.StatusBadRequest)
			return
		}
		pathHash := cookie.Value
		workspace := getWorkspace(r.Context(), pathHash)
		workspace.reverseProxy(w, r)
		return
	}

	pathHash := fmt.Sprintf("%x", md5.Sum([]byte(folder)))
	workspace := getWorkspace(r.Context(), pathHash)

	cookie := http.Cookie{Name: "pathHash", Value: workspace.pathHash, Path: "/"}
	http.SetCookie(w, &cookie)
	workspace.reverseProxy(w, r)
}

// getWorkspace returns a workspace for the given path hash. If the workspace
// doesn't exist, it will be created.
func getWorkspace(ctx context.Context, pathHash string) *Workspace {
	workspaceMapMu.Lock()
	defer workspaceMapMu.Unlock()

	if workspace, ok := workspaceMap[pathHash]; ok {
		return workspace
	}
	return createWorkspace(ctx, pathHash)
}

func createWorkspace(ctx context.Context, pathHash string) *Workspace {
	vscodeSocketPath := fmt.Sprintf("/tmp/vscode-%s.sock", pathHash)
	if _, err := os.Stat(vscodeSocketPath); err == nil {
		err = os.Remove(vscodeSocketPath)
		if err != nil {
			log.Fatalln("Error removing existing socket:", err)
		}
	}

	_, err := os.Create(vscodeSocketPath)
	if err != nil {
		log.Fatalln("Error creating socket:", err)
	}

	// Start a new child process for the folder
	cmd := exec.Command("code-server", "--socket", vscodeSocketPath, pathHash)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start child process: %v", err)
	}

	workspace := &Workspace{
		pathHash:         pathHash,
		vscodeSocketPath: vscodeSocketPath,
		process:          cmd.Process,
	}

	// Add the workspace to the map
	workspaceMap[pathHash] = workspace

	// Wait for the child process to exit and remove the workspace from the map
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("Child process terminated: %v", err)
			delete(workspaceMap, pathHash)
		}
	}()

	workspace.waitForSocket(ctx)

	return workspaceMap[pathHash]
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", proxyHandler)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
