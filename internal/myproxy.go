package internal

import (
	"context"
	"errors"
	"myproxy/internal/mlog"
	"myproxy/pkg/di"
	"myproxy/pkg/interfaces"
	"myproxy/pkg/models"
	"sync"
)

func New(iConfig *models.Config) (*Instance, error) {
	ctx, cancel := context.WithCancel(context.Background())
	instance := &Instance{Ctx: ctx, Cancel: cancel, Config: iConfig}

	err := instance.init()
	if err != nil {
		return nil, err
	}

	return instance, nil
}

type Instance struct {
	Lock    sync.Mutex
	Ctx     context.Context
	Cancel  context.CancelFunc
	Config  *models.Config
	Futures []interfaces.Future
	Running bool
}

func (i *Instance) init() error {
	configs := resolveConfig(i.Config)

	for _, config := range configs {
		o, err := di.GetServerInstance(i.Ctx, config)
		if err != nil {
			return err
		}

		if future, ok := o.(interfaces.Future); ok {
			if err = i.AddFuture(future); err != nil {
				return err
			}
		}
	}

	return nil
}

func (i *Instance) AddFuture(o interfaces.Future) error {
	i.Futures = append(i.Futures, o)
	if i.Running {
		if err := o.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (i *Instance) AddFutures(o []interfaces.Future) error {
	i.Futures = append(i.Futures, o...)
	if i.Running {
		for _, future := range o {
			if err := future.Run(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (i *Instance) Start() error {
	i.Lock.Lock()
	defer i.Lock.Unlock()

	i.Running = true

	for _, task := range i.Futures {
		if err := task.Run(); err != nil {
			return err
		}
	}

	mlog.Warn("Endpoint started")

	return nil
}

func (i *Instance) Close() error {
	i.Lock.Lock()
	defer i.Lock.Unlock()

	i.Running = false
	i.Cancel()

	var errMsg string
	for _, task := range i.Futures {
		if err := task.Close(); err != nil {
			errMsg += " " + err.Error()
		}
	}

	if errMsg != "" {
		return errors.New(errMsg)
	}

	return nil
}

func resolveConfig(cfg *models.Config) []any {
	cfgs := make([]any, 0)

	if cfg != nil {
		if cfg.Endpoint != nil {
			cfgs = append(cfgs, cfg.Endpoint)
		}
		if cfg.Outbounds != nil {
			cfgs = append(cfgs, cfg.Outbounds)
		}
		if cfg.Inbounds != nil {
			cfgs = append(cfgs, cfg.Inbounds)
		}
		if cfg.Routing != nil {
			cfgs = append(cfgs, cfg.Routing)
		}
	}

	return cfgs
}
