package lullaby

import (
	"context"
	"github.com/sourcegraph/conc"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

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

// Group manages graceful stopping of multiple services
type Group struct {
	wg              *conc.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
	stopOnce        sync.Once
	services        []Service
	startedServices []Service  // Track successfully started services
	mu              sync.Mutex // Protect startedServices
	timeout         time.Duration
}

// New creates a new Group with the specified timeout
func New(timeout time.Duration) *Group {
	ctx, cancel := context.WithCancel(context.Background())
	return &Group{
		wg:              conc.NewWaitGroup(),
		ctx:             ctx,
		cancel:          cancel,
		timeout:         timeout,
		services:        make([]Service, 0),
		startedServices: make([]Service, 0),
	}
}

// Add registers a service with the group
func (lg *Group) Add(service Service) {
	lg.services = append(lg.services, service)
}

func (lg *Group) Start() error {
	// Start signal handling
	lg.wg.Go(func() {
		lg.handleSignals()
	})

	// Start all services
	for _, service := range lg.services {
		srvc := service // Create new variable for closure
		lg.wg.Go(func() {
			// Track service as started before calling Start
			lg.mu.Lock()
			lg.startedServices = append(lg.startedServices, srvc)
			lg.mu.Unlock()

			if err := srvc.Start(lg.ctx); err != nil {
				lg.Stop() // Trigger stop on failure
			}
		})
	}

	return nil
}

// Wait blocks until all services have completed
func (lg *Group) Wait() {
	lg.wg.Wait()
}

// Stop initiates graceful stop of all services
func (lg *Group) Stop() {
	lg.stopOnce.Do(func() {
		lg.cancel()
		lg.stopAll()
	})
}

// handleSignals sets up signal handling for graceful stop
func (lg *Group) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-lg.ctx.Done():
		return
	case <-sigChan:
		lg.Stop()
	}
}

// stopAll gracefully stops all successfully started services
func (lg *Group) stopAll() {
	stopCtx, cancel := context.WithTimeout(context.Background(), lg.timeout)
	defer cancel()

	// Create a WaitGroup for stop operations
	stopWg := conc.NewWaitGroup()

	// Get the list of services to stop under lock
	lg.mu.Lock()
	servicesToStop := lg.startedServices
	lg.mu.Unlock()

	// Stop each service that was successfully started
	for _, service := range servicesToStop {
		srvc := service // Create new variable for closure
		stopWg.Go(func() {
			_ = srvc.Stop(stopCtx)
		})
	}

	stopWg.Wait()
}
