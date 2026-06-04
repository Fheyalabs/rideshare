module github.com/Fheyalabs/rideshare

go 1.23.3

require github.com/Fheyalabs/ares-core v0.7.5

// Local dev: consume the sibling checkout. Drop once ares-core is a tagged published dep.
replace github.com/Fheyalabs/ares-core => ../ARES-core
