package workspace

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"regexp"
	"time"
)

type Workspace struct {
	Socket       string
	VscodeSocket string
	Process      *os.Process
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
			req.URL.Path = regexp.MustCompile(`^/ws/`).ReplaceAllString(r.URL.Path, "/")
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

func WaitForSocket(ctx context.Context, socket string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	backoff := time.Millisecond * 100
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := net.Dial("unix", socket)
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
	st, err := workspace.Process.Wait()
	if err != nil {
		log.Println("Error waiting for process to exit:", err)
	}
	log.Printf("Workspace exited with status %d\n", st.ExitCode())

	os.Remove(workspace.Socket)
}

func DecodePathFromKey(key string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func (w *Workspace) OpenFile(file string, line int, column int) {
	fileURI := fmt.Sprintf("%s:%d:%d", file, line, column)

	jsonData, err := json.Marshal(map[string]interface{}{
		"type":             "open",
		"folderURIs":       []string{},
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
				return net.Dial("unix", w.VscodeSocket)
			},
		},
	}
	resp, err := httpc.Post("http://unix/", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("Error sending JSON to server: %v\n", err)
		return
	}

	// Print response:
	respData, err := httputil.DumpResponse(resp, true)
	if err != nil {
		log.Fatalln("Error dumping response:", err)
	}
	log.Printf("Response: %s\n", respData)
	defer resp.Body.Close()
}

func CreateWorkspace() *Workspace {
	codeServerSocketPath := GetCodeServerSocketPath()

	_, err := os.Create(codeServerSocketPath)
	if err != nil {
		log.Fatalln("Error creating socket:", err)
	}

	cmd := exec.Command("code-server", "--socket", codeServerSocketPath)
	// cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Prevent child process from being killed when parent process exits
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err = cmd.Start(); err != nil {
		log.Fatalf("Failed to start child process: %v", err)
	}

	err = WaitForSocket(context.Background(), codeServerSocketPath)
	if err != nil {
		log.Printf("Failed health check for child process: %v", err)
	}

	// Do not block on vscode socket creation
	w := &Workspace{
		Socket:  codeServerSocketPath,
		Process: cmd.Process,
	}

	return w
}

// Wait for a minute before timing out.
func WaitForVscodeSocket(pid int) chan string {
	doneChan := make(chan string)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		for {
			vscodeSocketPath, err := GetVscodeSocketPath(pid)
			if err == nil {
				doneChan <- vscodeSocketPath
				close(doneChan)
				break
			}
			select {
			case <-ctx.Done():
				log.Println("Timed out waiting for vscode-ipc socket path")
				close(doneChan)
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
	}()
	return doneChan
}

func GetCodeServerSocketPath() string {
	return fmt.Sprintf("%scode-server.sock", os.TempDir())
}

// Reads the vscode-ipc socket path from $TMPDIR/vscode-ipc
func GetVscodeSocketPath(pid int) (string, error) {
	b, err := os.ReadFile(fmt.Sprintf("%svscode-ipc-%d", os.TempDir(), pid))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func GetFolderFromSocketPath(socketPath string) (string, error) {
	pattern := regexp.MustCompile(`code-server-(.+)\.sock$`)
	matches := pattern.FindStringSubmatch(socketPath)
	if len(matches) != 2 {
		return "", fmt.Errorf("invalid socket path: %s", socketPath)
	}
	return DecodePathFromKey(matches[1])
}
