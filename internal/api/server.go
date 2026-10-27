package api

import (
	"net/http"
	"time"
)

func NewServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:				":8080",
		Handler: 			handler,
		ReadHeaderTimeout:	5*time.Second,
	}
}