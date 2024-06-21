package control

import (
	"context"
	"myproxy/internal/proxy"
	"myproxy/pkg/di"
	"myproxy/pkg/models"
	"reflect"
)

type inboundServer struct {
	Ctx      context.Context
	Inbounds []*models.Inbound
}

func (i *inboundServer) Run() error {
	for _, inbound := range i.Inbounds {
		go proxy.Process(i.Ctx, inbound)
	}
	return nil
}

func (i *inboundServer) Close() error {
	return nil
}

func inboundServerCreator(ctx context.Context, v any) (any, error) {
	inbounds := v.([]*models.Inbound)
	return &inboundServer{Ctx: ctx, Inbounds: inbounds}, nil
}

func init() {
	lc := reflect.TypeOf([]*models.Inbound{})
	di.ServerContext[lc] = inboundServerCreator
}
