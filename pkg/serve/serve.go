package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/hostrouter"
	"github.com/google/uuid"
	"github.com/sleexyz/dev-world/pkg/sitter"
	"github.com/soheilhy/cmux"
)

type Event struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type App struct {
	sitter      *sitter.Sitter
	subscribers map[string]chan Event
}

func createApp() *App {
	return &App{
		sitter:      sitter.LoadSitter(),
		subscribers: make(map[string]chan Event),
	}
}

func (a *App) openFile(file string, line int, column int) {
	// Shell out to code-server
	ws := a.sitter.GetWorkspaceForFile(file)
	if ws == nil {
		log.Printf("No workspace found for file %s\n", file)
		return
	}
	ws.OpenFile(file, line, column)

	// Broadcast to extension
	for _, sub := range a.subscribers {
		sub <- Event{
			File:   file,
			Line:   line,
			Column: column,
		}
	}
}

func (a *App) ListenOpenFileSSE(w http.ResponseWriter, r *http.Request) {
	log.Printf("New subscriber\n")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// Generate a random id for this subscriber
	id := uuid.New().String()
	if _, ok := a.subscribers[id]; ok {
		log.Printf("Subscriber with id %s already exists\n", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
	sub := make(chan Event)
	a.subscribers[id] = sub
	defer delete(a.subscribers, id)
	for {
		select {
		case event := <-sub:
			w.Write([]byte("data: "))
			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("Error marshalling event: %s\n", err)
				return
			}
			w.Write(data)
			w.Write([]byte("\n\n"))
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			log.Printf("Subscriber with id %s disconnected\n", id)
			return
		}
	}
}

func (a *App) HandleOpenFile(w http.ResponseWriter, r *http.Request) {
	var event Event
	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		log.Printf("Error decoding request body: %s\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	a.openFile(event.File, event.Line, event.Column)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
}

func (a *App) ProxyToCodeServer(w http.ResponseWriter, r *http.Request) {
	a.sitter.ProxyHandler(w, r)
}

func (a *App) ProxyToFrontend(w http.ResponseWriter, r *http.Request) {
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "localhost:12344"
			req.URL.Path = r.URL.Path
		},
	}
	proxy.ServeHTTP(w, r)
}

type GetWorkspacesResponse struct {
	Workspaces []string `json:"workspaces"`
}

func (a *App) HandleGetWorkspaces(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ws := a.sitter.GetWorkspacePaths()
	resp := &GetWorkspacesResponse{
		Workspaces: ws,
	}
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Printf("Error encoding response: %s\n", err)
	}
}

func (a *App) HandleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("folder")
	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err := a.sitter.DeleteWorkspace(path)
	if err != nil {
		log.Printf("Error deleting workspace: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	a.HandleGetWorkspaces(w, r)
}

func (a *App) RedirectToWorkspace(w http.ResponseWriter, r *http.Request) {
	home := os.Getenv("HOME")
	r.URL.Query().Get("alias")
	path := filepath.Join(home, r.URL.Query().Get("alias"))
	log.Printf("Redirecting to %s\n", path)
	http.Redirect(w, r, "https://dev.localhost:12345?folder="+path, http.StatusTemporaryRedirect)
}

func (app *App) makeCodeServerRouter() chi.Router {
	r := chi.NewRouter()
	r.NotFound(app.ProxyToCodeServer)
	return r
}

func (app *App) makeFrontendRouter() chi.Router {
	r := chi.NewRouter()
	r.Get("/workspace", app.RedirectToWorkspace)
	r.Get("/api/workspaces", app.HandleGetWorkspaces)
	r.Post("/api/open-file", app.HandleOpenFile)
	r.Delete("/api/workspace", app.HandleDeleteWorkspace)
	r.Get("/api/listen-open-file", app.ListenOpenFileSSE)
	r.NotFound(app.ProxyToFrontend)
	return r
}

func main() {
	flag.Parse()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Exiting 1...")
		os.Exit(0)
	}()
	port := os.Getenv("PORT")
	if port == "" {
		port = "12345"
	}

	app := createApp()
	hr := hostrouter.New()

	codeServerRouter := app.makeCodeServerRouter()
	frontendRouter := app.makeFrontendRouter()
	hr.Map("localhost:12345", frontendRouter)
	hr.Map("dev.localhost:12345", codeServerRouter)
	hr.Map("d", frontendRouter)
	hr.Map("dev", frontendRouter)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Mount("/", hr)

	// Create a custom handler function for CONNECT requests
	connectHandler := func(w http.ResponseWriter, r *http.Request) {
		// Establish a plain-text TCP connection to the target server
		conn, err := net.Dial("tcp", "localhost:12345")
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		// Return a 200 OK response to the client
		w.WriteHeader(http.StatusOK)
		// Hijack the client connection to establish a bidirectional stream with the target server
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
			return
		}
		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Copy data bidirectionally between the client and the target server
		go func() {
			defer conn.Close()
			defer clientConn.Close()
			_, err := io.Copy(conn, clientConn)
			if err != nil {
				log.Printf("Error copying to client: %s\n", err)
			}
		}()
		go func() {
			defer conn.Close()
			defer clientConn.Close()
			_, err := io.Copy(clientConn, conn)
			if err != nil {
				log.Printf("Error copying to server: %s\n", err)
			}
		}()
	}

	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}

	m := cmux.New(l)
	httpsL := m.Match(cmux.TLS())
	httpL := m.Match(cmux.Any())

	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Redirect to HTTPS:
			target := "https://" + r.Host + r.URL.Path
			if len(r.URL.RawQuery) > 0 {
				target += "?" + r.URL.RawQuery
			}
			log.Printf("Redirecting to %s\n", target)
			http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		}),
	}
	httpsServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				log.Printf("Handling CONNECT request for %s\n", r.Host)
				connectHandler(w, r)
			} else {
				log.Printf("Handling %s request for %s\n", r.Method, r.URL.Path)
				router.ServeHTTP(w, r)
			}
		}),
		// Force HTTP/1.1 to enable hijacking
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true, // Disabling certificate verification, use with caution.
		},
	}

	log.Printf("Listening on port %s\n", port)
	go func() {
		err := httpServer.Serve(httpL)
		if err != nil {
			log.Fatal(err)
		}
	}()
	go func() {
		err := httpsServer.ServeTLS(httpsL, "localhost+4.pem", "localhost+4-key.pem")
		if err != nil {
			log.Fatal(err)
		}
	}()
	m.Serve()
}
