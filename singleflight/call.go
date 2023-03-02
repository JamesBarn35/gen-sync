package singleflight

import (
	"sync"
)

// call is an in-flight or completed singleflight.Do call
type call[B any] struct {
	waitGroup sync.WaitGroup

	// These fields are written once before the WaitGroup is done
	// and are only read after the WaitGroup is done.
	val B
	err error

	// These fields are read and written with the singleflight
	// mutex held before the WaitGroup is done, and are read but
	// not written after the WaitGroup is done.
	dups  int
	chans []chan<- ResultContainer[B]
}
