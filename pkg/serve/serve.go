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
	"github.com/google/uuid"
	"github.com/sleexyz/dev-world/pkg/workspace"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc/test/bufconn"
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

func Tunnel(from, to io.ReadWriteCloser) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered while tunneling")
		}
	}()

	io.Copy(from, to)
	to.Close()
	from.Close()
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

	loopback := bufconn.Listen(1024)

	handleTLS := func(clientConn net.Conn) {
		tlsconn, ok := clientConn.(*tls.Conn)
		if ok {

			err := tlsconn.Handshake()
			if err != nil {
				log.Printf("error in tls.Handshake: %s", err)
				clientConn.Close()
				return
			}

			backendConn, err := loopback.Dial()
			if err != nil {
				log.Printf("error in net.Dial: %s", err)
				clientConn.Close()
				return
			}

			go Tunnel(clientConn, backendConn)
			go Tunnel(backendConn, clientConn)
		}
	}

	// Create a custom handler function for CONNECT requests
	connectHandler := func(w http.ResponseWriter, r *http.Request) {
		// Establish a plain-text TCP connection to the target server
		// conn, err := net.Dial("tcp", "localhost:12345")
		conn, err := loopback.DialContext(r.Context())
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
		go Tunnel(conn, clientConn)
		go Tunnel(clientConn, conn)
	}

	go func() {
		l, err := net.Listen("tcp", ":"+port)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Listening on :%s\n", port)
		http.Serve(l, LoggerMiddleware("tcp")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				connectHandler(w, r)
			} else {
				target := "https://" + r.Host + r.URL.Path
				if len(r.URL.RawQuery) > 0 {
					target += "?" + r.URL.RawQuery
				}
				log.Printf("Redirecting to %s\n", target)
				http.Redirect(w, r, target, http.StatusTemporaryRedirect)
			}
		})))
	}()

	go func() {
		m := cmux.New(loopback)
		httpsL := m.Match(cmux.TLS())
		httpL := m.Match(cmux.Any())

		go func() {
			err := http.Serve(httpL, LoggerMiddleware("memory")(router))
			if err != nil {
				log.Fatal(err)
			}
		}()
		go func() {
			cert, err := tls.LoadX509KeyPair(*certFileFlag, *keyFileFlag)
			if err != nil {
				log.Fatalf("error in tls.LoadX509KeyPair: %s", err)
			}

			listener := tls.NewListener(httpsL, &tls.Config{Certificates: []tls.Certificate{cert}, InsecureSkipVerify: true})

			for {
				conn, err := listener.Accept()
				if err != nil {
					log.Printf("error in listener.Accept: %s", err)
					break
				}

				go handleTLS(conn)
			}
		}()
		m.Serve()
	}()
	go func() {
		<-c
		log.Println("Exiting 1...")
		app.Close()
		os.Exit(0)
	}()
	<-make(chan struct{})
}
