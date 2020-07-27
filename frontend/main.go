package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
)

var noserve bool

func init() {
	flag.BoolVar(&noserve, "ns", false, "Disable the HTTP server")
}

func main() {
	flag.Parse()

	sh(`cd src      && \
		go generate && \
		go build -o ../bin/main.wasm .`)

	if !noserve {
		log.Println("Listening at: 127.0.0.1:42070")
		log.Fatalln(http.ListenAndServe("127.0.0.1:42070", http.FileServer(http.Dir("./bin"))))
	}
}

func sh(cmd string) {
	c := exec.Command("sh", "-c", cmd)
	c.Stdin = os.Stdin
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	c.Env = append(os.Environ(),
		"GOOS=js",
		"GOARCH=wasm",
	)

	if err := c.Run(); err != nil {
		log.Fatalln(err)
	}
}
