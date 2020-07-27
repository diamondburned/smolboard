package jsfn

import (
	"encoding/json"
	"log"

	"github.com/vugu/vugu/js"
)

// SetLocalStorage marshals v into the local storage with k.
func SetLocalStorage(k string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		log.Println("Failed to marshal for local storage:", err)
		return err
	}

	localStorage := js.Global().Get("window").Get("localStorage")
	localStorage.Call("setItem", k, string(b))

	return nil
}

// GetLocalStorage gets the JSON from k and unmarshals into v.
func GetLocalStorage(k string, v interface{}) error {
	localStorage := js.Global().Get("window").Get("localStorage")
	b := []byte(localStorage.Call("getItem", k).String())

	if err := json.Unmarshal(b, v); err != nil {
		log.Println("Failed to unmarshal for local storage:", err)
		return err
	}

	return nil
}
