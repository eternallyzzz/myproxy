package control

import (
	"context"
	"myproxy/internal/router"
	"myproxy/pkg/di"
	"myproxy/pkg/models"
	"reflect"
)

type routeServer struct {
	Ctx     context.Context
	Routing *models.Routing
}

func (r *routeServer) Run() error {
	router.Run(r.Routing.Rules)
	return nil
}

func (r *routeServer) Close() error {
	return nil
}

func routeServerCreator(ctx context.Context, v any) (any, error) {
	routing := v.(*models.Routing)
	return &routeServer{Ctx: ctx, Routing: routing}, nil
}

func init() {
	rc := reflect.TypeOf(&models.Routing{})
	di.ServerContext[rc] = routeServerCreator
}
