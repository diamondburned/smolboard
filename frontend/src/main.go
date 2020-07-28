package main

import (
	"github.com/diamondburned/smolboard/client"
)

func main() {
	var session = client.NewSession("")

	a, err := NewApp(session)
	if err != nil {
		panic(err)
	}

	if err := a.Main(); err != nil {
		panic(err)
	}
}
