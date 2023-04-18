package workspace

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"time"
)

type Workspace struct {
	Key                  string
	Folder               string
	CodeServerSocketPath string
	Process              *os.Process
}

func (workspace *Workspace) ReverseProxy(w http.ResponseWriter, r *http.Request) {
	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", workspace.CodeServerSocketPath)
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
			conn, err := net.Dial("unix", workspace.CodeServerSocketPath)
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
