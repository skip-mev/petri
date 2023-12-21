module github.com/skip-mev/petri/types

go 1.21.3

replace (
	github.com/skip-mev/petri/provider => ../provider
	github.com/skip-mev/petri/util => ../util
)

require github.com/skip-mev/petri/provider v0.0.0-00010101000000-000000000000

require github.com/skip-mev/petri/util v0.0.0-00010101000000-000000000000 // indirect
