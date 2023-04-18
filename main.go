package main

import (
	"log"
	"net/http"
	"os"

	"github.com/sleexyz/dev-world/pkg/sitter"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	sitter := sitter.InitializeSitter()
	http.HandleFunc("/", sitter.ProxyHandler)
	log.Printf("Listening on port %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
