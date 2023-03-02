package singleflight

import (
	"bytes"
	"errors"
	"fmt"
	"runtime/debug"
)

// errGoexit indicates the runtime.Goexit was called in
// the user given function.
var errGoexit = errors.New("runtime.Goexit was called")

// A panicError is an arbitrary value recovered from a panic
// with the stack trace during the execution of given function.
type panicError[B any] struct {
	value B
	stack []byte
}

// Error implements error interface.
func (p *panicError[B]) Error() string {
	return fmt.Sprintf("%v\n\n%s", p.value, p.stack)
}

func newPanicError[B any](b B) error {
	stack := debug.Stack()

	// The first line of the stack trace is of the form "goroutine N [status]:"
	// but by the time the panic reaches Do the goroutine may no longer exist
	// and its status will have changed. Trim out the misleading line.
	if line := bytes.IndexByte(stack[:], '\n'); line >= 0 {
		stack = stack[line+1:]
	}
	return &panicError[B]{value: b, stack: stack}
}
