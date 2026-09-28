package main

import (
	"flag"
	"log"
	"net/http"

	"nydus/internal/fakeapi"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	seed := flag.Int64("seed", 42, "rng seed")
	flag.Parse()

	s := fakeapi.NewServer(*seed)
	http.HandleFunc("/records", s.Records)
	http.HandleFunc("/control", s.Control)
	http.HandleFunc("/healthz", s.Healthz)

	log.Printf("fakeapi listening on %s", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}
