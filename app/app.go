package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir("/"))))
	mux.Handle("/c/", http.StripPrefix("/c/", http.FileServer(http.Dir("c:"))))
	mux.Handle("/d/", http.StripPrefix("/d/", http.FileServer(http.Dir("d:"))))
	mux.Handle("/e/", http.StripPrefix("/e/", http.FileServer(http.Dir("e:"))))
	if err := http.ListenAndServe(":3008", mux); err != nil {
		log.Fatal(err)
	}
}