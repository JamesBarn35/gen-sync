package singleflight

import (
	"runtime"
	"sync"
)

// Group represents a class of work and forms a namespace in
// which units of work can be executed with duplicate suppression.
type Group[A comparable, B any] struct {
	mu         sync.Mutex
	workspaces workNamespaces[A, B]
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time. If a duplicate comes in, the duplicate caller waits for the
// original to complete and receives the same results.
// The return value shared indicates whether v was given to multiple callers.
func (g *Group[A, B]) Do(
	key A,
	fn supplier[B],
) (B, error, bool) {
	g.mu.Lock()
	if g.workspaces == nil {
		g.workspaces = make(map[A]*call[B])
	}
	if c, ok := g.workspaces[key]; ok {
		c.dups++
		g.mu.Unlock()
		c.waitGroup.Wait()

		if e, ok := c.err.(*panicError[B]); ok {
			panic(e)
		} else if c.err == errGoexit {
			runtime.Goexit()
		}
		return c.val, c.err, true
	}
	c := new(call[B])
	c.waitGroup.Add(1)
	g.workspaces[key] = c
	g.mu.Unlock()

	g.doCall(c, key, fn)
	return c.val, c.err, c.dups > 0
}

// DoChan is like Do but returns a channel that will receive the
// results when they are ready.
//
// The returned channel will not be closed.
func (g *Group[A, B]) DoChan(
	key A,
	fn supplier[B],
) <-chan ResultContainer[B] {
	ch := make(chan ResultContainer[B], 1)
	g.mu.Lock()
	if g.workspaces == nil {
		g.workspaces = make(map[A]*call[B])
	}
	if c, ok := g.workspaces[key]; ok {
		c.dups++
		c.chans = append(c.chans, ch)
		g.mu.Unlock()
		return ch
	}
	c := &call[B]{chans: []chan<- ResultContainer[B]{ch}}
	c.waitGroup.Add(1)
	g.workspaces[key] = c
	g.mu.Unlock()

	go g.doCall(c, key, fn)

	return ch
}

// doCall handles the single call for a key.
func (g *Group[A, B]) doCall(
	c *call[B],
	key A,
	fn supplier[B],
) {
	normalReturn := false
	recovered := false

	// use double-defer to distinguish panic from runtime.Goexit,
	// more details see https://golang.org/cl/134395
	defer func() {
		// the given function invoked runtime.Goexit
		if !normalReturn && !recovered {
			c.err = errGoexit
		}

		g.mu.Lock()
		defer g.mu.Unlock()
		c.waitGroup.Done()
		if g.workspaces[key] == c {
			delete(g.workspaces, key)
		}

		if e, ok := c.err.(*panicError[B]); ok {
			// In order to prevent the waiting channels from being blocked forever,
			// needs to ensure that this panic cannot be recovered.
			if len(c.chans) > 0 {
				go panic(e)
				select {} // Keep this goroutine around so that it will appear in the crash dump.
			} else {
				panic(e)
			}
		} else if c.err == errGoexit {
			// Already in the process of goexit, no need to call again
		} else {
			// Normal return
			for _, ch := range c.chans {
				ch <- ResultContainer[B]{c.val, c.err, c.dups > 0}
			}
		}
	}()

	func() {
		defer func() {
			if !normalReturn {
				// Ideally, we would wait to take a stack trace until we've determined
				// whether this is a panic or a runtime.Goexit.
				//
				// Unfortunately, the only way we can distinguish the two is to see
				// whether the recover stopped the goroutine from terminating, and by
				// the time we know that, the part of the stack trace relevant to the
				// panic has been discarded.
				if r := recover(); r != nil {
					c.err = newPanicError(r)
				}
			}
		}()

		c.val, c.err = fn()
		normalReturn = true
	}()

	if !normalReturn {
		recovered = true
	}
}

// Forget tells the singleflight to forget about a key.  Future calls
// to Do for this key will call the function rather than waiting for
// an earlier call to complete.
func (g *Group[A, B]) Forget(key A) {
	g.mu.Lock()
	delete(g.workspaces, key)
	g.mu.Unlock()
}
