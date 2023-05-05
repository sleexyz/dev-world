package main

import (
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
	"github.com/google/uuid"
	"github.com/sleexyz/dev-world/pkg/workspace"
	"github.com/soheilhy/cmux"
)

const (
	yellow = "\033[33m"
	green  = "\033[32m"
	reset  = "\033[0m"
)

var (
	certFileFlag = flag.String("cert-file", "cert.pem", "path to cert file")
	keyFileFlag  = flag.String("key-file", "key.pem", "path to key file")
)

type Event struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type App struct {
	ws          *workspace.Workspace
	subscribers map[string]chan Event
}

func createApp() *App {
	return &App{
		ws:          workspace.CreateWorkspace(),
		subscribers: make(map[string]chan Event),
	}
}

func (a *App) openFile(file string, line int, column int) {
	// Shell out to code-server
	a.ws.OpenFile(file, line, column)

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
	a.ws.ReverseProxy(w, r)
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

func (a *App) RedirectToWorkspace(w http.ResponseWriter, r *http.Request) {
	home := os.Getenv("HOME")
	r.URL.Query().Get("alias")
	path := filepath.Join(home, r.URL.Query().Get("alias"))
	log.Printf("Redirecting to %s\n", path)
	http.Redirect(w, r, "/ws/?folder="+path, http.StatusTemporaryRedirect)
}

func LoggerMiddleware(category string) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodString := r.Method
			if r.Method == http.MethodConnect {
				methodString = green + "CONNECT" + reset
			}
			log.Printf(
				"[%s%6s%s] (%s) %s %s\n",
				yellow,
				category,
				reset,
				r.Proto,
				methodString,
				r.RequestURI,
			)
			h.ServeHTTP(w, r)
		})
	}
}

func (app *App) makeFrontendRouter() chi.Router {
	r := chi.NewRouter()
	r.Get("/workspace", app.RedirectToWorkspace)
	r.Post("/api/open-file", app.HandleOpenFile)
	r.Get("/api/listen-open-file", app.ListenOpenFileSSE)
	r.Mount("/ws", http.HandlerFunc(app.ProxyToCodeServer))
	r.NotFound(app.ProxyToFrontend)
	return r
}

func (app *App) Close() {
	app.ws.Close()
}

func main() {
	flag.Parse()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	port := os.Getenv("PORT")
	if port == "" {
		port = "12345"
	}

	app := createApp()

	router := app.makeFrontendRouter()

	// iml := fasthttputil.NewInmemoryListener()

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
		Handler: LoggerMiddleware("http")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				connectHandler(w, r)
			} else {
				// Redirect to HTTPS:
				target := "https://" + r.Host + r.URL.Path
				if len(r.URL.RawQuery) > 0 {
					target += "?" + r.URL.RawQuery
				}
				log.Printf("Redirecting to %s\n", target)
				http.Redirect(w, r, target, http.StatusTemporaryRedirect)
			}
		})),
	}
	httpsServer := &http.Server{
		Handler: LoggerMiddleware("https")(router),
	}

	log.Printf("Listening on port %s\n", port)
	go func() {
		err := httpServer.Serve(httpL)
		if err != nil {
			log.Fatal(err)
		}
	}()
	go func() {
		err := httpsServer.ServeTLS(httpsL, *certFileFlag, *keyFileFlag)
		if err != nil {
			log.Fatal(err)
		}
	}()
	go func() {
		<-c
		log.Println("Exiting 1...")
		app.Close()
		os.Exit(0)
	}()
	m.Serve()
}
