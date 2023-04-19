package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/sleexyz/dev-world/pkg/sitter"
)

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
	sitter := sitter.InitializeSitter()
	http.HandleFunc("/", sitter.ProxyHandler)
	log.Printf("Listening on port %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
