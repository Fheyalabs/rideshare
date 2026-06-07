package main

import (
	"flag"
	"log"
	"os"

	"github.com/Fheyalabs/rideshare/internal/routing"
)

func main() {
	in := flag.String("in", "", "OSM .pbf path (Saxony + N. Bohemia extract)")
	out := flag.String("out", "region.bin", "serialized region output")
	flag.Parse()
	if *in == "" {
		log.Fatal("-in required")
	}
	g, err := routing.ParsePBF(*in)
	if err != nil {
		log.Fatalf("parse: %v", err)
	}
	b, err := routing.Serialize(g)
	if err != nil {
		log.Fatalf("serialize: %v", err)
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		log.Fatalf("write: %v", err)
	}
	log.Printf("wrote %s: %d nodes, %d edges", *out, len(g.Nodes), len(g.Edges))
}
