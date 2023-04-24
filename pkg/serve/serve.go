package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"

	"github.com/go-chi/chi/v5"
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

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("folder") == "" {
		if r.URL.Path == "/" {
			// Unset cookie:
			http.SetCookie(w, &http.Cookie{
				Name:   sitter.WORKSPACE_PATH_COOKIE,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
		}
		cookieValue, _ := r.Cookie(sitter.WORKSPACE_PATH_COOKIE)
		if r.URL.Path == "/" || cookieValue == nil {
			proxy := &httputil.ReverseProxy{
				Director: func(req *http.Request) {
					req.URL.Scheme = "http"
					req.URL.Host = "localhost:12344"
					req.URL.Path = r.URL.Path
				},
			}
			proxy.ServeHTTP(w, r)
			return
		}
	}
	a.sitter.ProxyHandler(w, r)
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
	r := chi.NewRouter()
	r.Get("/__api__/workspaces", app.GetWorkspaces)
	r.Delete("/__api__/workspace", app.DeleteWorkspace)
	r.NotFound(app.ServeHTTP)

	http.ListenAndServe(":"+port, r)
	log.Printf("Listening on port %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
