package main

import (
	"io"
	"log"
	"net/http"
)

func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, "Internal Server Error")
				log.Printf("panic: %v", err)
			}

		}()
		next.ServeHTTP(w, r)
	})
}
