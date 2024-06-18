package common

import (
	"context"
	"errors"
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
	typeOf := reflect.TypeOf(cfg)
	creator, ok := ServerContext[typeOf]
	if !ok {
		return nil, errors.New(typeOf.String() + "is not found")
	}

	return creator(ctx, cfg)
}
