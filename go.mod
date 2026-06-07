module github.com/Fheyalabs/rideshare

go 1.23.3

require (
	github.com/Fheyalabs/ares-core v0.7.5
	github.com/paulmach/osm v0.9.0
	github.com/uber/h3-go/v4 v4.3.0
)

require (
	github.com/DataDog/czlib v0.0.0-20240814115052-86a9592b3985 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/paulmach/orb v0.12.0 // indirect
	github.com/paulmach/protoscan v0.2.1 // indirect
	go.mongodb.org/mongo-driver v1.17.4 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

// Local dev: consume the sibling checkout. Drop once ares-core is a tagged published dep.
replace github.com/Fheyalabs/ares-core => ../ARES-core
