package workspace

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"time"
)

type Workspace struct {
	Path    string `json:"path"`
	Socket  string `json:"socket"`
	Process *os.Process
}

func (workspace *Workspace) ReverseProxy(w http.ResponseWriter, r *http.Request) error {
	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", workspace.Socket)
		},
	}

	errorChan := make(chan error, 1)
	doneChan := make(chan struct{})
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = r.Host
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if err != nil {
				errorChan <- err
			}
		},
	}

	go func() {
		proxy.ServeHTTP(w, r)
		doneChan <- struct{}{}
	}()

	select {
	case <-doneChan:
		return nil
	case err := <-errorChan:
		return err
	}
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
			conn, err := net.Dial("unix", workspace.Socket)
			if err == nil {
				conn.Close()
				return nil
			}
			log.Println("Server is not alive, waiting...")
			time.Sleep(backoff)
			backoff *= 2
		}
	}
}

func (workspace *Workspace) Close() {
	workspace.Process.Kill()
	os.Remove(workspace.Socket)
}

func CreateKeyFromFolder(folder string) string {
	return base64.StdEncoding.EncodeToString([]byte(folder))
}

func (w *Workspace) OpenFile(file string, line int, column int) {
	fileURI := fmt.Sprintf("%s:%d:%d", file, line, column)

	jsonData, err := json.Marshal(map[string]interface{}{
		"type":             "open",
		"fileURIs":         []string{fileURI},
		"forceReuseWindow": true,
		"gotoLineMode":     true,
	})
	if err != nil {
		log.Fatalln("Error marshalling JSON:", err)
	}
	log.Printf("Sending JSON: %s to server at %s\n", jsonData, w.Socket)
	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				// TODO: write to the vscode-ipc socket instead of the code-server socket.
				return net.Dial("unix", w.Socket)
			},
		},
	}
	resp, err := httpc.Do(&http.Request{
		Method: "POST",
		URL:    &url.URL{Scheme: "http", Host: "unix", Path: "/"},
		Body:   io.NopCloser(bytes.NewReader(jsonData)),
	})
	if err != nil {
		log.Fatalln("Error sending request:", err)
	}

	// Print response:
	respData, err := httputil.DumpResponse(resp, true)
	if err != nil {
		log.Fatalln("Error dumping response:", err)
	}
	log.Printf("Response: %s\n", respData)

	defer resp.Body.Close()
}

func CreateWorkspace(ctx context.Context, path string) *Workspace {
	codeServerSocketPath := GetCodeServerSocketPath(path)

	_, err := os.Create(codeServerSocketPath)
	if err != nil {
		log.Fatalln("Error creating socket:", err)
	}

	// Start a new child process for the folder
	cmd := exec.Command("code-server", "--socket", codeServerSocketPath, path)
	// cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Prevent child process from being killed when parent process exits
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start child process: %v", err)
	}

	workspace := &Workspace{
		Path:    path,
		Socket:  codeServerSocketPath,
		Process: cmd.Process,
	}

	err = workspace.WaitForSocket(ctx)
	if err != nil {
		log.Printf("Failed health check for child process: %v", err)
	}

	return workspace
}

func GetCodeServerSocketPath(folder string) string {
	key := CreateKeyFromFolder(folder)
	return fmt.Sprintf("%scode-server-%s.sock", os.TempDir(), key)
}
