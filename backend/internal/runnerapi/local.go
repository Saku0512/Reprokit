package runnerapi

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
)

// LocalServer はWebシェルが同一端末で利用するloopback専用のRunner APIである。
type LocalServer struct {
	Listener     net.Listener
	Handler      http.Handler
	SessionToken string
}

func NewLocalServer(service Service) (*LocalServer, error) {
	token, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen on loopback: %w", err)
	}
	return &LocalServer{Listener: listener, Handler: NewHandler(service, token), SessionToken: token}, nil
}

func newSessionToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate local session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}
