package client

import (
	"net/http"
	"time"
)

type Session struct {
	http.Client
}

func NewSession() *Session {
	return &Session{
		Client: http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *Session) Login(username, password string)
