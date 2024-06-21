package di

import (
	"context"
	"errors"
	_ "myproxy/internal/control"
	"reflect"
)

var (
	ServerContext = make(map[reflect.Type]Creator)
)

type Creator func(ctx context.Context, v any) (any, error)

func GetServerInstance(ctx context.Context, cfg any) (any, error) {
	if cfg == nil {
		return nil, errors.New("config: invalid memory address or nil pointer dereference")
	}

	t := reflect.TypeOf(cfg)
	creator, ok := ServerContext[t]

	if !ok {
		return nil, errors.New(t.String() + " is not found")
	}

	return creator(ctx, cfg)
}
