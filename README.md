# threadgroup

[![GoDoc](https://godoc.org/github.com/lukechampine/threadgroup?status.svg)](https://godoc.org/github.com/lukechampine/threadgroup) [![Go Report Card](https://goreportcard.com/badge/github.com/lukechampine/threadgroup)](https://goreportcard.com/report/github.com/lukechampine/threadgroup)

Package threadgroup exposes a ThreadGroup object which can be used to
facilitate clean shutdown. A ThreadGroup is similar to a sync.WaitGroup,
but with two important additions: The ability to detect when shutdown has
been initiated, and protections against adding more threads after shutdown
has completed.

ThreadGroup was designed with the following shutdown sequence in mind:

1. Call Stop, signaling that shutdown has begun. After Stop is called, no
new goroutines should be created.

2. Wait for Stop to return. When Stop returns, all goroutines should have
returned.

3. Free any resources used by the goroutines.

## Example

```go
type MyListener struct {
	listener net.Listener
	log      *log.Logger
	tg       threadgroup.ThreadGroup
}

func (l *MyListener) Start() error {
	if l.tg.IsStopped() {
		return nil
	}

	for {
		conn, err := l.listener.Accept()
		if err != nil {
			break
		}
		go l.handleConn(conn)
	}
	return nil
}

func (l *MyListener) handleConn(net.Conn) {
	if l.tg.IsStopped() {
		return
	}

	select {
	case <-time.After(time.Minute):
		l.log.Println("finished")
		return
	case <-l.tg.StopChan():
		l.log.Println("exiting early")
		return
	}
}

func (l *MyListener) Close() error {
	// first, prevent new goroutines from being spawned
	err := l.listener.Close()
	if err != nil {
		return err
	}
	// then, signal shutdown and wait for all goroutines to return
	l.tg.Stop()
	// finally, close resources used by the goroutines
	return l.log.Close()
}
```
