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
	startCtx        context.Context
	cancelStart     context.CancelFunc
	stopOnce        sync.Once
	services        []Service
	startedServices []Service
	timeout         time.Duration
}

func New(timeout time.Duration) *Group {
	startCtx, cancelStart := context.WithCancel(context.Background())
	return &Group{
		wg:              conc.NewWaitGroup(),
		startCtx:        startCtx,
		cancelStart:     cancelStart,
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
	// Start the service in a goroutine but wait for any error
	errCh := make(chan error, 1)
	lg.wg.Go(func() {
		err := service.Start(lg.startCtx)
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

	lg.startedServices = append(lg.startedServices, service)
	return nil
}

func (lg *Group) Wait() {
	lg.wg.Wait()
}

func (lg *Group) Stop() {
	lg.stopOnce.Do(func() {
		lg.cancelStart()
		lg.stopAll()
	})
}

func (lg *Group) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-lg.startCtx.Done():
		return
	case <-sigChan:
		lg.Stop()
	}
}

func (lg *Group) stopAll() {
	stopCtx, cancel := context.WithTimeout(context.Background(), lg.timeout)
	defer cancel()

	// Stop services sequentially in reverse order
	for i := len(lg.startedServices) - 1; i >= 0; i-- {
		_ = lg.startedServices[i].Stop(stopCtx)
	}
}
