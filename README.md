# lullaby

`lullaby` is a Go package that helps you gracefully manage service lifecycles. Built on top of [sourcegraph/conc](https://github.com/sourcegraph/conc), it embraces structured concurrency patterns to coordinate the starting and stopping of multiple services while handling system signals automatically.

## Features

- 🚀 Simple API for service lifecycle management
- 🔄 Coordinated service startup and shutdown
- ⚡ Concurrent service stopping with structured concurrency (via `sourcegraph/conc`)
- ⏰ Configurable timeout for stopping services
- 🛑 Automatic stopping of all services if any fails during initialization
- 📡 Built-in system signal handling (SIGINT, SIGTERM)
- 🔒 Thread-safe stopping mechanism

## Why lullaby?

### Built on Structured Concurrency

`lullaby` is built on top of `sourcegraph/conc`, which brings structured concurrency patterns to Go. This means:
- Better goroutine lifecycle management
- Automatic panic handling
- Cleaner, more maintainable concurrent code
- Built-in error propagation

### Fail-Fast with Grace

If any service fails during initialization, `lullaby` automatically initiates a graceful stop of all other services that started successfully. This ensures your system fails predictably and cleanly:

```go
group := lullaby.New(5 * time.Second)
group.Add(service1)
group.Add(service2) // If service2 fails to start
group.Add(service3)

group.Start() // service1 will be stopped gracefully
```

### Flexible Stopping Mechanisms

While `lullaby` handles SIGINT/SIGTERM automatically, it also exposes a `Stop()` method for programmatic control. This is useful for:
- Testing scenarios
- Custom shutdown triggers (e.g., health checks, remote commands)
- Business logic-driven shutdowns

```go
// Signal-based stopping (automatic)
group.Start()

// Programmatic stopping
group.Start()
if someCondition {
    group.Stop() // Manually trigger stop
}
```

## Installation

```bash
go get github.com/ibrhmkoz/lullaby
```

## Quick Start

```go
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
}
```

## Service Interface

Your services need to implement the `lullaby.Service` interface:

```go
type Startable interface {
    Start(context.Context) error
}

type Stoppable interface {
    Stop(context.Context) error
}

type Service interface {
    Startable
    Stoppable
}
```

## Example

See [examples/basic/main.go](examples/basic/main.go) for a complete example.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE for details.

## Credits

This package is built on top of [sourcegraph/conc](https://github.com/sourcegraph/conc), which provides excellent structured concurrency primitives for Go.
