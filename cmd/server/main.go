package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Fheyalabs/rideshare/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()
	srv := server.New(server.Config{Addr: *addr})
	log.Printf("rideshare server listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
