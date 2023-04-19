package workspace

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Workspace struct {
	Key        string
	SocketPath string
	Process    *os.Process
}

func (workspace *Workspace) ReverseProxy(w http.ResponseWriter, r *http.Request) {
	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", workspace.SocketPath)
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

func (workspace *Workspace) WaitForSocket(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	backoff := time.Millisecond * 100
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := net.Dial("unix", workspace.SocketPath)
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

func CreateKeyFromFolder(folder string) string {
	return base64.StdEncoding.EncodeToString([]byte(folder))
}

func CreateWorkspace(ctx context.Context, folder string) *Workspace {
	key := CreateKeyFromFolder(folder)
	codeServerSocketPath := fmt.Sprintf("/tmp/code-server-%s.sock", key)

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

	workspace := &Workspace{
		Key:        key,
		SocketPath: codeServerSocketPath,
		Process:    cmd.Process,
	}

	err = workspace.WaitForSocket(ctx)
	if err != nil {
		log.Printf("Failed health check for child process: %v", err)
	}

	return workspace
}
