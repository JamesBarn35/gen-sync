package singleflight

type supplier[B any] func() (B, error)
type workNamespaces[A comparable, B any] map[A]*call[B]
