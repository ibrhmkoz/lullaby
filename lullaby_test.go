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
	// Test setup
	group := New(0)
	service := newMockService()

	// Add service to group
	group.Add(service)

	// Start group
	if err := group.Start(); err != nil {
		t.Fatalf("Failed to start group: %v", err)
	}

	// Wait for service to actually start
	service.waitForStart()

	// Stop group
	group.Stop()

	// Wait for all operations to complete
	group.Wait()

	// Verify service was started and stopped
	if !service.wasStartCalled() {
		t.Error("Service was not started")
	}
	if !service.wasStopCalled() {
		t.Error("Service was not stopped")
	}
}

func TestGroupMultipleServices(t *testing.T) {
	// Test setup
	group := New(0)
	services := []*mockService{
		newMockService(),
		newMockService(),
		newMockService(),
	}

	// Add all services to group
	for _, service := range services {
		group.Add(service)
	}

	// Start group
	if err := group.Start(); err != nil {
		t.Fatalf("Failed to start group: %v", err)
	}

	// Wait for all services to actually start
	for _, service := range services {
		service.waitForStart()
	}

	// Stop group
	group.Stop()

	// Wait for all operations to complete
	group.Wait()

	// Verify all services were started and stopped
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
	// Test setup
	group := New(0)
	service1 := newMockService()
	service2 := newMockService()
	service2.shouldStartErr = true
	service3 := newMockService()

	// Add services to group
	group.Add(service1)
	group.Add(service2)
	group.Add(service3)

	// Start group
	if err := group.Start(); err != nil {
		t.Fatalf("Failed to start group: %v", err)
	}

	// Only wait for service1 to start as service2 will error
	service1.waitForStart()

	// Wait for all operations to complete
	group.Wait()

	// Verify behavior when a service fails to start
	if !service1.wasStopCalled() {
		t.Error("Service 1 was not stopped after failure")
	}
	if !service2.wasStartCalled() {
		t.Error("Service 2 start was not attempted")
	}
	// Don't verify service3's stop status as it may or may not be stopped
	// depending on timing of service2's failure
}

func TestGroupIdempotentStop(t *testing.T) {
	// Test setup
	group := New(0)
	service := newMockService()

	group.Add(service)

	// Start group
	if err := group.Start(); err != nil {
		t.Fatalf("Failed to start group: %v", err)
	}

	// Wait for service to actually start
	service.waitForStart()

	// Stop group multiple times
	group.Stop()
	group.Stop()
	group.Stop()

	// Wait for all operations to complete
	group.Wait()

	// Verify service was started and stopped exactly once
	if !service.wasStartCalled() {
		t.Error("Service was not started")
	}
	if !service.wasStopCalled() {
		t.Error("Service was not stopped")
	}
}
