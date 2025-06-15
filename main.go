package main

import (
	"hexbot/router"
	"log"
	"net/http"
	"runtime/debug"
)


func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				msg := "Caught Panic: %v\nSTACK TRACE: %s\n"
				log.Printf(msg, err, stack)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
        next.ServeHTTP(w, r)
    })
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/",router.ApiHandler)
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("💥 boom")
	})

	wrappedMux := RecoveryMiddleware(mux)

	log.Println("starting server on port :8080")
	log.Fatal(http.ListenAndServe(":8080", wrappedMux))
}
