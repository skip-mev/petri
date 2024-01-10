module github.com/skip-mev/petri/monitoring

go 1.21.3

replace github.com/skip-mev/petri/provider => ../provider

replace github.com/skip-mev/petri/util => ../util

require (
	github.com/go-rod/rod v0.114.5
	github.com/skip-mev/petri/provider v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.26.0
)

require (
	github.com/skip-mev/petri/util v0.0.0-00010101000000-000000000000 // indirect
	github.com/ysmood/fetchup v0.2.4 // indirect
	github.com/ysmood/goob v0.4.0 // indirect
	github.com/ysmood/got v0.39.3 // indirect
	github.com/ysmood/gson v0.7.3 // indirect
	github.com/ysmood/leakless v0.8.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
)
