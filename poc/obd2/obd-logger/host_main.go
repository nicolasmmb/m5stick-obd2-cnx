//go:build !tinygo

// This file exists only so `go build ./...` works on the host. Real firmware
// entry points live in main.go and are built by TinyGo.
package main

func main() {}
