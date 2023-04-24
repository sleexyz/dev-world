package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/hostrouter"
	"github.com/sleexyz/dev-world/pkg/sitter"
)

type App struct {
	sitter *sitter.Sitter
}

func createApp() *App {
	return &App{
		sitter: sitter.InitializeSitter(),
	}
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

func (a *App) GetWorkspaces(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ws := a.sitter.GetWorkspaces()
	resp := &GetWorkspacesResponse{
		Workspaces: ws,
	}
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Printf("Error encoding response: %s\n", err)
	}
}

func (a *App) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("folder")
	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err := a.sitter.DeleteWorkspace(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	a.GetWorkspaces(w, r)
}

func (a *App) RedirectToWorkspace(w http.ResponseWriter, r *http.Request) {
	home := os.Getenv("HOME")
	r.URL.Query().Get("alias")
	path := filepath.Join(home, r.URL.Query().Get("alias"))
	log.Printf("Redirecting to %s\n", path)
	http.Redirect(w, r, "http://dev.localhost:12345?folder="+path, http.StatusTemporaryRedirect)
}

func (app *App) makeCodeServerRouter() chi.Router {
	r := chi.NewRouter()
	r.NotFound(app.ProxyToCodeServer)
	return r
}

func (app *App) makeFrontendRouter() chi.Router {
	r := chi.NewRouter()
	r.Get("/workspace", app.RedirectToWorkspace)
	r.Get("/__api__/workspaces", app.GetWorkspaces)
	r.Delete("/__api__/workspace", app.DeleteWorkspace)
	r.NotFound(app.ProxyToFrontend)
	return r
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Exiting 1...")
		os.Exit(0)
		log.Println("Exited. This should not print.")
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

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Mount("/", hr)

	http.ListenAndServe(":"+port, r)
	log.Printf("Listening on port %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
