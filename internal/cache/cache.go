
package cache

import "context"


type Client interface {
	Name() string
	Get(ctx context.Context, key string) (value []byte, ok bool, err error)
	Set(ctx context.Context, key string, value []byte) error
	Del(ctx context.Context, keys ...string) error
	Flush(ctx context.Context) error
}
