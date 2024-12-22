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

type Group struct {
	wg       *conc.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	services []Service
	timeout  time.Duration
}

func New(timeout time.Duration) *Group {
	ctx, cancel := context.WithCancel(context.Background())
	return &Group{
		wg:       conc.NewWaitGroup(),
		ctx:      ctx,
		cancel:   cancel,
		timeout:  timeout,
		services: make([]Service, 0),
	}
}

func (lg *Group) Add(service Service) {
	lg.services = append(lg.services, service)
}

func (lg *Group) Start() error {
	// Start signal handling
	lg.wg.Go(lg.handleSignals)

	// Start all services concurrently using conc's WaitGroup
	for _, service := range lg.services {
		srv := service // Create new variable for closure
		lg.wg.Go(func() {
			if err := srv.Start(lg.ctx); err != nil {
				lg.Stop() // Trigger stop on failure
			}
		})
	}

	return nil
}

func (lg *Group) Wait() {
	lg.wg.Wait()
}

func (lg *Group) Stop() {
	lg.stopOnce.Do(func() {
		lg.cancel()
		lg.stopServices()
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
