module github.com/Fheyalabs/rideshare

go 1.23.3

require (
	github.com/Fheyalabs/ares-core v0.7.5
	github.com/uber/h3-go/v4 v4.3.0
)

require github.com/gorilla/websocket v1.5.3 // indirect

// Local dev: consume the sibling checkout. Drop once ares-core is a tagged published dep.
replace github.com/Fheyalabs/ares-core => ../ARES-core
