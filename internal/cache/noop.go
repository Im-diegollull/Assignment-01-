package cache

import "context"

type noop struct{}

// Noop nunca guarda nada Get siempre es miss
func Noop() Client { return noop{} }

func (noop) Name() string                                       { return "off" }
func (noop) Get(context.Context, string) ([]byte, bool, error) { return nil, false, nil }
func (noop) Set(context.Context, string, []byte) error         { return nil }
func (noop) Del(context.Context, ...string) error              { return nil }
func (noop) Flush(context.Context) error                       { return nil }
