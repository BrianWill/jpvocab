package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	"fmt"
	"log"
	"net/http"
	"time"
)

func serverInit() {
	r := chi.NewRouter()

	r.Get("/", mainRoute)
	r.Get("/hello-world", helloWorld)

	log.Printf("Starting server on http://localhost:%d", port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), r); err != nil {
		panic(err)
	}
}

func mainRoute(w http.ResponseWriter, r *http.Request) {
	w.Write(helloWorldHTML)
}

func helloWorld(w http.ResponseWriter, r *http.Request) {
	const message = "Hello, world!"
	type Store struct {
		Delay time.Duration `json:"delay"`
	}
	store := &Store{}
	if err := datastar.ReadSignals(r, store); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	for i := 0; i < len(message); i++ {
		if err := sse.PatchElements(`<div id="message">` + message[:i+1] + `</div>`); err != nil {
			return
		}
		time.Sleep(store.Delay * time.Millisecond)
	}
}
