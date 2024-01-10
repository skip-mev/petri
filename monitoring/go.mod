module github.com/skip-mev/petri/monitoring

go 1.21.3

replace github.com/skip-mev/petri/provider => ../provider

replace github.com/skip-mev/petri/util => ../util

require github.com/skip-mev/petri/provider v0.0.0-00010101000000-000000000000

require (
	github.com/go-rod/rod v0.114.5 // indirect
	github.com/skip-mev/petri/util v0.0.0-00010101000000-000000000000 // indirect
	github.com/stretchr/objx v0.5.1 // indirect
	github.com/ysmood/fetchup v0.2.4 // indirect
	github.com/ysmood/goob v0.4.0 // indirect
	github.com/ysmood/got v0.39.3 // indirect
	github.com/ysmood/gson v0.7.3 // indirect
	github.com/ysmood/leakless v0.8.0 // indirect
)
