// Package threadgroup exposes a ThreadGroup object which can be used to
// facilitate clean shutdown. A ThreadGroup is similar to a sync.WaitGroup,
// but with two important additions: The ability to detect when shutdown has
// been initiated, and protections against adding more threads after shutdown
// has completed.
//
// ThreadGroup was designed with the following shutdown sequence in mind:
//
// 1. Call Stop, signaling that shutdown has begun. After Stop is called, no
// new goroutines should be created.
//
// 2. Wait for Stop to return. When Stop returns, all goroutines should have
// returned.
//
// 3. Free any resources used by the goroutines.
package threadgroup

import (
	"context"
	"sync"
)

// A ThreadGroup is a sync.WaitGroup with additional functionality for
// facilitating clean shutdown. Namely, it provides a StopChan method for
// notifying callers when shutdown occurs. Another key difference is that a
// ThreadGroup is only intended be used once; as such, its Add and Stop
// methods return false if Stop has already been called.
//
// During shutdown, it is common to close resources such as net.Listeners.
// Typically, this would require spawning a goroutine to wait on the
// ThreadGroup's StopChan and then close the resource. To make this more
// convenient, ThreadGroup provides an OnStop method. Functions passed to
// OnStop will be called automatically when Stop is called.
type ThreadGroup struct {
	stopChan chan struct{}
	chanOnce sync.Once
	mu       sync.Mutex
	wg       sync.WaitGroup
}

// StopChan returns a channel that is closed when Stop is called. Callers
// should select on StopChan in order to interrupt long-running operations
// (such as time.After).
func (tg *ThreadGroup) StopChan() <-chan struct{} {
	tg.chanOnce.Do(func() { tg.stopChan = make(chan struct{}) })
	return tg.stopChan
}

// IsStopped returns true if Stop has been called.
func (tg *ThreadGroup) IsStopped() bool {
	select {
	case <-tg.StopChan():
		return true
	default:
		return false
	}
}

// Add increments the ThreadGroup counter and returns true if Stop has not yet
// been called. Otherwise, it returns false and does nothing.
func (tg *ThreadGroup) Add() bool {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	if tg.IsStopped() {
		return false
	}
	tg.wg.Add(1)
	return true
}

// Done decrements the ThreadGroup counter. As with sync.WaitGroup, calling
// Done when the counter is zero will result in a panic.
func (tg *ThreadGroup) Done() {
	tg.wg.Done()
}

// Stop closes the ThreadGroup's StopChan and blocks until the counter is
// zero. After the StopChan has been closed, calls to Add will return false.
func (tg *ThreadGroup) Stop() bool {
	tg.mu.Lock()
	if tg.IsStopped() {
		tg.mu.Unlock()
		return false
	}
	close(tg.stopChan)
	tg.mu.Unlock()
	tg.wg.Wait()
	return true
}

// OnStop is a convenience function that blocks until the ThreadGroup is
// stopped, and then calls fn. It is intended to be used to close resources
// during shutdown, e.g.:
//
//  var tg ThreadGroup
//  l, _ := net.Listen("tcp", ":0")
//  go tg.OnStop(func() { l.Close() })
//  for {
//  	conn, err := l.Accept()
//  	if err != nil {
//  		break
//  	}
//  	go handleConn(conn, &tg)
//  }
//
// In this example, when tg.Stop is called, the listener will be closed,
// causing l.Accept to return an error and thus preventing the creation of new
// goroutines. Note that if OnStop is called in a goroutine, it may be
// necessary to call Add/Done inside fn to ensure that Stop does not return
// until fn has returned. Furthermore, note that the caller is responsible for
// the ordering of multiple OnStop functions.
func (tg *ThreadGroup) OnStop(fn func()) {
	<-tg.StopChan()
	fn()
}

// NewContext returns a Context that is canceled either when parent is canceled
// or when Stop is called.
func (tg *ThreadGroup) NewContext(parent context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-tg.StopChan():
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}
