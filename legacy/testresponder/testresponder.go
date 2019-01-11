package testresponder

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/ndt-server/legacy/tcplistener"
)

// Message constants for use in their respective channels
const (
	Ready = float64(-1)
)

// Config expresses the configuration of the server, and whether to use TLS or not.
type Config struct {
	KeyFile, CertFile string
	TLS               bool
}

// TestResponder coordinates synchronization between the main control loop and subtests.
type TestResponder struct {
	Response chan float64
	Port     int
	Ln       net.Listener
	S        *http.Server
	Ctx      context.Context
	Cancel   context.CancelFunc
	Config   *Config
}

// MakeNdtUpgrader creates a websocket Upgrade for the NDT legacy
func MakeNdtUpgrader(protocols []string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:    81920,
		WriteBufferSize:   81920,
		Subprotocols:      protocols,
		EnableCompression: false,
		CheckOrigin: func(r *http.Request) bool {
			// TODO: make this check more appropriate -- added to get initial html5 widget to work.
			return true
		},
	}
}

// Listen on a random port.
func listenRandom() (net.Listener, int, error) {
	// Start listening
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{})
	if err != nil {
		return nil, 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	return tcplistener.RawListener{TCPListener: ln}, port, nil
}

// StartAsync allocates a new TLS HTTP server listening on a random port. The
// server can be stopped again using TestResponder.Close().
func (tr *TestResponder) StartAsync(mux *http.ServeMux, msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	tr.Ctx = ctx
	tr.Cancel = cancel
	tr.Response = make(chan float64)
	ln, port, err := listenRandom()
	if err != nil {
		log.Println("ERROR: Failed to listen on any port:", err)
		return err
	}
	tr.Port = port
	tr.Ln = ln
	tr.S = &http.Server{Handler: mux}
	go func() {
		log.Printf("%s: Serving for test on %s", msg, ln.Addr())
		var err error
		if tr.Config.TLS {
			err = tr.S.ServeTLS(ln, tr.Config.CertFile, tr.Config.KeyFile)
		} else {
			err = tr.S.Serve(ln)
		}
		if err != nil && err != http.ErrServerClosed {
			log.Printf("ERROR: %s Starting server: %s", msg, err)
		}
	}()
	return nil
}

// Close will shutdown, cancel, or close all resources used by the test.
func (tr *TestResponder) Close() {
	log.Println("Closing Test Responder")
	if tr.S != nil {
		// Shutdown the server for the test.
		tr.S.Close()
	}
	if tr.Ln != nil {
		// Shutdown the socket listener.
		tr.Ln.Close()
	}
	if tr.Cancel != nil {
		// Cancel the test responder context.
		tr.Cancel()
	}
	// Close channel for communication between the control routine and test routine.
	close(tr.Response)
}