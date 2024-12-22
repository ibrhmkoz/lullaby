package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"lullaby"
	"net/http"
	"time"
)

// HTTPServer wraps http.Server to implement the Shutdownable interface
type HTTPServer struct {
	*http.Server
	name string
}

func NewHTTPServer(name, addr string, handler http.Handler) *HTTPServer {
	return &HTTPServer{
		Server: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
		name: name,
	}
}

func (s *HTTPServer) Start(ctx context.Context) error {
	log.Printf("Starting %s on %s", s.name, s.Addr)
	if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("%s error: %v", s.name, err)
		return err
	}
	return nil
}

func (s *HTTPServer) Stop(ctx context.Context) error {
	log.Printf("Shutting down %s", s.name)
	return s.Server.Shutdown(ctx)
}

func main() {
	// Create servers
	server1 := NewHTTPServer("Server 1", ":56999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello from Server 1 on port 56999!")
	}))

	server2 := NewHTTPServer("Server 2", ":57000", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello from Server 2 on port 57000!")
	}))

	// Create shutdown group with 5-second timeout
	group := lullaby.New(5 * time.Second)

	// Add servers to the group
	group.Add(server1)
	group.Add(server2)

	// Start all servers
	if err := group.Start(); err != nil {
		log.Fatal(err)
	}

	// Wait for shutdown
	group.Wait()
	log.Println("All servers stopped gracefully")
}
