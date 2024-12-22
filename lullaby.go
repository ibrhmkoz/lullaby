package lullaby

import (
	"context"
	"fmt"
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

type Group struct {
	wg          *conc.WaitGroup
	startCtx    context.Context
	cancelStart context.CancelFunc
	stopOnce    sync.Once
	services    []Service
	timeout     time.Duration
}

func New(timeout time.Duration) *Group {
	startCtx, cancelStart := context.WithCancel(context.Background())
	return &Group{
		wg:          conc.NewWaitGroup(),
		startCtx:    startCtx,
		cancelStart: cancelStart,
		timeout:     timeout,
		services:    make([]Service, 0),
	}
}

func (lg *Group) Add(service Service) {
	lg.services = append(lg.services, service)
}

func (lg *Group) Start() error {
	errChan := make(chan error, len(lg.services))
	lg.wg.Go(lg.handleSignals)

	for _, service := range lg.services {
		srv := service
		lg.wg.Go(func() {
			if err := srv.Start(lg.startCtx); err != nil {
				errChan <- err
				lg.Stop()
			}
		})
	}

	select {
	case err := <-errChan:
		return fmt.Errorf("service start failed: %w", err)
	default:
		return nil
	}
}

func (lg *Group) Wait() {
	lg.wg.Wait()
}

func (lg *Group) Stop() {
	lg.stopOnce.Do(func() {
		lg.cancelStart()
		lg.stopServices()
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

func (lg *Group) stopServices() {
	// Create a new WaitGroup for stopping
	stopWg := conc.NewWaitGroup()
	defer stopWg.Wait()

	// Create timeout context for stopping
	stopCtx, cancel := context.WithTimeout(context.Background(), lg.timeout)
	defer cancel()

	// Stop all services concurrently using conc
	for _, service := range lg.services {
		srv := service // Create new variable for closure
		stopWg.Go(func() {
			_ = srv.Stop(stopCtx)
		})
	}
}
