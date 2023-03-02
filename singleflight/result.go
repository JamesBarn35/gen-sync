package singleflight

// Result holds the results of Do
// so they can be passed on a channel.
type ResultContainer[B any] struct {
	value  B
	Err    error
	Shared bool
}
