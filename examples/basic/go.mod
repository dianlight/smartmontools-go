module example

go 1.24.7

replace github.com/dianlight/smartmontools-go => ../..

require (
	github.com/dianlight/smartmontools-go v0.0.0-00010101000000-000000000000
	github.com/fatih/color v1.18.0
)

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.25.0 // indirect
)
