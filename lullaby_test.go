package lullaby

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// mockService implements Service interface for testing
type mockService struct {
	mu             sync.Mutex
	startCalled    bool
	stopCalled     bool
	shouldStartErr bool
	shouldStopErr  bool
	started        chan struct{} // signals when Start has begun executing
}

func newMockService() *mockService {
	return &mockService{
		started: make(chan struct{}),
	}
}

func (m *mockService) Start(ctx context.Context) error {
	m.mu.Lock()
	m.startCalled = true
	m.mu.Unlock()

	if m.shouldStartErr {
		return errors.New("start error")
	}

	// Signal that Start has begun executing
	close(m.started)

	// Block until context is cancelled
	<-ctx.Done()
	return nil
}

func (m *mockService) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopCalled = true
	if m.shouldStopErr {
		return errors.New("stop error")
	}
	return nil
}

func (m *mockService) wasStartCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startCalled
}

func (m *mockService) wasStopCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopCalled
}

func (m *mockService) waitForStart() {
	<-m.started
}

func TestGroupBasicOperation(t *testing.T) {
	group := New(0)
	service := newMockService()
	group.Add(service)

	// Start in goroutine since it blocks
	errChan := make(chan error, 1)
	go func() {
		errChan <- group.Start()
	}()

	// Wait for service to actually start
	service.waitForStart()

	// Trigger stop
	group.Stop()

	// Check start error
	if err := <-errChan; err != nil {
		t.Fatalf("Failed to start group: %v", err)
	}

	if !service.wasStartCalled() {
		t.Error("Service was not started")
	}
	if !service.wasStopCalled() {
		t.Error("Service was not stopped")
	}
}

func TestGroupMultipleServices(t *testing.T) {
	group := New(0)
	services := []*mockService{
		newMockService(),
		newMockService(),
		newMockService(),
	}

	for _, service := range services {
		group.Add(service)
	}

	// Start in goroutine since it blocks
	errChan := make(chan error, 1)
	go func() {
		errChan <- group.Start()
	}()

	// Wait for all services to actually start
	for _, service := range services {
		service.waitForStart()
	}

	// Trigger stop
	group.Stop()

	// Check start error
	if err := <-errChan; err != nil {
		t.Fatalf("Failed to start group: %v", err)
	}

	for i, service := range services {
		if !service.wasStartCalled() {
			t.Errorf("Service %d was not started", i)
		}
		if !service.wasStopCalled() {
			t.Errorf("Service %d was not stopped", i)
		}
	}
}

func TestGroupFailedStart(t *testing.T) {
	group := New(0)
	service1 := newMockService()
	service2 := newMockService()
	service2.shouldStartErr = true
	service3 := newMockService()

	group.Add(service1)
	group.Add(service2)
	group.Add(service3)

	// Start should return error
	err := group.Start()
	if err == nil {
		t.Fatal("Expected error from Start(), got nil")
	}

	if !service1.wasStopCalled() {
		t.Error("Service 1 was not stopped after failure")
	}
	if !service2.wasStartCalled() {
		t.Error("Service 2 start was not attempted")
	}
}

func TestGroupIdempotentStop(t *testing.T) {
	group := New(0)
	service := newMockService()
	group.Add(service)

	// Start in goroutine since it blocks
	errChan := make(chan error, 1)
	go func() {
		errChan <- group.Start()
	}()

	// Wait for service to actually start
	service.waitForStart()

	// Stop group multiple times
	group.Stop()
	group.Stop()
	group.Stop()

	// Check start error
	if err := <-errChan; err != nil {
		t.Fatalf("Failed to start group: %v", err)
	}

	if !service.wasStartCalled() {
		t.Error("Service was not started")
	}
	if !service.wasStopCalled() {
		t.Error("Service was not stopped")
	}
}
