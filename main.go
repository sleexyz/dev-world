package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type Workspace struct {
	pathHash string
	port     string
	process  *os.Process
}

var (
	// workspaceMap maps path hashes to workspaces.
	workspaceMap   = make(map[string]*Workspace)
	workspaceMapMu sync.Mutex
)

func (workspace *Workspace) reverseProxy(w http.ResponseWriter, r *http.Request) {
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = fmt.Sprintf("localhost:%s", workspace.port)
			req.Host = r.Host
		},
	}
	proxy.ServeHTTP(w, r)
}

func (workspace *Workspace) waitForHealthCheck() error {
	url := fmt.Sprintf("http://localhost:%s/healthz", workspace.port)
	maxBackoff := time.Second * 5
	backoff := time.Millisecond * 100
	for {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			var data struct{ Alive bool }
			err = json.NewDecoder(resp.Body).Decode(&data)
			resp.Body.Close()
			if err == nil && data.Alive {
				fmt.Println("Server is alive!")
				return nil
			}
		}
		fmt.Println("Server is not alive, waiting...")
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			return fmt.Errorf("timeout reached, server is not responding")
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
		workspace := getWorkspace(pathHash)
		workspace.reverseProxy(w, r)
		return
	}

	pathHash := fmt.Sprintf("%x", md5.Sum([]byte(folder)))
	workspace := getWorkspace(pathHash)

	cookie := http.Cookie{Name: "pathHash", Value: workspace.pathHash, Path: "/"}
	http.SetCookie(w, &cookie)
	workspace.reverseProxy(w, r)
}

func findPort() string {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("Failed to listen on a random port: %v", err)
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	ln.Close()
	return port
}

// getWorkspace returns a workspace for the given path hash. If the workspace
// doesn't exist, it will be created.
func getWorkspace(pathHash string) *Workspace {
	workspaceMapMu.Lock()
	defer workspaceMapMu.Unlock()

	if workspace, ok := workspaceMap[pathHash]; ok {
		return workspace
	}

	// Find an available port
	port := findPort()

	// Start a new child process for the folder
	cmd := exec.Command("code-server", "--port", port, pathHash)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start child process: %v", err)
	}

	workspace := &Workspace{
		pathHash: pathHash,
		port:     port,
		process:  cmd.Process,
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

	workspace.waitForHealthCheck()

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
