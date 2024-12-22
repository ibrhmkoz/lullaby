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
	startedServices []Service
	mu              sync.Mutex
	timeout         time.Duration
}

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

func (lg *Group) Add(service Service) {
	lg.services = append(lg.services, service)
}

func (lg *Group) Start() error {
	// Start signal handling in a separate goroutine
	lg.wg.Go(func() {
		lg.handleSignals()
	})

	// Start services sequentially in the order they were added
	for _, service := range lg.services {
		if err := lg.startService(service); err != nil {
			lg.Stop() // Trigger stop on failure
			return err
		}
	}

	return nil
}

func (lg *Group) startService(service Service) error {
	// Track service before starting
	lg.mu.Lock()
	lg.startedServices = append(lg.startedServices, service)
	lg.mu.Unlock()

	// Start the service in a goroutine but wait for any error
	errCh := make(chan error, 1)
	lg.wg.Go(func() {
		err := service.Start(lg.ctx)
		if err != nil {
			errCh <- err
		}
		close(errCh)
	})

	// Wait for immediate errors before proceeding to next service
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-time.After(100 * time.Millisecond): // Brief window to catch startup errors
	}

	return nil
}

func (lg *Group) Wait() {
	lg.wg.Wait()
}

func (lg *Group) Stop() {
	lg.stopOnce.Do(func() {
		lg.cancel()
		lg.stopAll()
	})
}

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

func (lg *Group) stopAll() {
	stopCtx, cancel := context.WithTimeout(context.Background(), lg.timeout)
	defer cancel()

	// Get the list of services to stop under lock
	lg.mu.Lock()
	servicesToStop := make([]Service, len(lg.startedServices))
	copy(servicesToStop, lg.startedServices)
	lg.mu.Unlock()

	// Stop services sequentially in reverse order
	for i := len(servicesToStop) - 1; i >= 0; i-- {
		_ = servicesToStop[i].Stop(stopCtx)
	}
}
