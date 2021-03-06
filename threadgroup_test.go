package threadgroup

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// TestThreadGroup tests normal operation of a ThreadGroup.
func TestThreadGroup(t *testing.T) {
	var tg ThreadGroup
	for i := 0; i < 10; i++ {
		if !tg.Add() {
			t.Fatal("Add failed?")
		}

		go func() {
			defer tg.Done()
			select {
			case <-time.After(1 * time.Second):
			case <-tg.StopChan():
			}
		}()
	}
	start := time.Now()
	ok := tg.Stop()
	elapsed := time.Since(start)
	if !ok {
		t.Fatal("Already stopped?")
	} else if elapsed > 100*time.Millisecond {
		t.Fatal("Stop did not interrupt goroutines")
	}
}

// TestStop tests the behavior of a ThreadGroup after Stop has been called.
func TestStop(t *testing.T) {
	var tg ThreadGroup

	// IsStopped should return false
	if tg.IsStopped() {
		t.Error("IsStopped returns true on unstopped ThreadGroup")
	}

	if !tg.Stop() {
		t.Fatal("Already stopped?")
	}

	// IsStopped should return true
	if !tg.IsStopped() {
		t.Error("IsStopped returns false on stopped ThreadGroup")
	}

	// Add and Stop should return false
	if tg.Add() {
		t.Error("Add succeeded")
	}
	if tg.Stop() {
		t.Error("Stop succeeded")
	}
}

// TestConcurrentAdd tests that Add can be called concurrently with Stop.
func TestConcurrentAdd(t *testing.T) {
	var tg ThreadGroup
	for i := 0; i < 10; i++ {
		go func() {
			if !tg.Add() {
				return
			}
			defer tg.Done()

			select {
			case <-time.After(1 * time.Second):
			case <-tg.StopChan():
			}
		}()
	}
	time.Sleep(10 * time.Millisecond) // wait for at least one Add
	if !tg.Stop() {
		t.Fatal("Already stopped?")
	}
}

// TestOnce tests that a zero-valued ThreadGroup's stopChan is properly
// initialized.
func TestOnce(t *testing.T) {
	tg := new(ThreadGroup)
	if tg.stopChan != nil {
		t.Error("expected nil stopChan")
	}

	// these methods should cause stopChan to be initialized
	tg.StopChan()
	if tg.stopChan == nil {
		t.Error("stopChan should have been initialized by StopChan")
	}

	tg = new(ThreadGroup)
	tg.IsStopped()
	if tg.stopChan == nil {
		t.Error("stopChan should have been initialized by IsStopped")
	}

	tg = new(ThreadGroup)
	tg.Add()
	if tg.stopChan == nil {
		t.Error("stopChan should have been initialized by Add")
	}

	tg = new(ThreadGroup)
	tg.Stop()
	if tg.stopChan == nil {
		t.Error("stopChan should have been initialized by Stop")
	}
}

// TestOnStop tests that Stop calls functions registered with
// OnStop.
func TestOnStop(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	// create ThreadGroup and register the closer
	var tg ThreadGroup
	go tg.OnStop(func() { l.Close() })

	// send on channel when listener is closed
	var closed bool
	tg.Add()
	go func() {
		defer tg.Done()
		_, err := l.Accept()
		closed = err != nil
	}()

	tg.Stop()
	if !closed {
		t.Fatal("Stop did not close listener")
	}
}

// TestClosedOnStop tests that OnStop returns immediately if the ThreadGroup
// has already been stopped.
func TestClosedOnStop(t *testing.T) {
	var tg ThreadGroup
	tg.Stop()
	closed := false
	tg.OnStop(func() { closed = true })
	if !closed {
		t.Fatal("close function should have been called immediately")
	}
}

// TestNewContext tests that contexts created by NewContext are canceled when
// their associated ThreadGroup is stopped.
func TestNewContext(t *testing.T) {
	var tg ThreadGroup
	ctx := tg.NewContext(context.Background())
	tg.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(1 * time.Second):
		t.Fatal("context should have been canceled")
	}
}

// TestRace tests that calling ThreadGroup methods concurrently does not
// trigger the race detector.
func TestRace(t *testing.T) {
	var tg ThreadGroup
	go tg.IsStopped()
	go tg.StopChan()
	go func() {
		if tg.Add() {
			tg.Done()
		}
	}()
	tg.Stop()
}

func BenchmarkThreadGroup(b *testing.B) {
	var tg ThreadGroup
	for i := 0; i < b.N; i++ {
		tg.Add()
		go tg.Done()
	}
	tg.Stop()
}

func BenchmarkWaitGroup(b *testing.B) {
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go wg.Done()
	}
	wg.Wait()
}
