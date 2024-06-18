package core

import (
	"context"
	"errors"
	"sync"
	_ "testDemo/core/control"
	"testDemo/pkg/common"
	"testDemo/pkg/config"
	"testDemo/pkg/inf"
	"testDemo/pkg/model"
	"testDemo/pkg/zlog"
)

func New(iConfig *model.Config) (*Instance, error) {
	if iConfig.EndPoint == nil && iConfig.Inbounds == nil {
		return nil, errors.New("invalid config")
	}

	ctx, cancel := context.WithCancel(context.Background())
	instance := &Instance{Ctx: ctx, Cancel: cancel}

	err := initInstance(instance, iConfig)
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func initInstance(ins *Instance, iConfig *model.Config) error {
	for _, role := range iConfig.Role {
		switch role {
		case config.RoleSrv:
			o, err := common.GetServerInstance(ins.Ctx, iConfig.EndPoint)
			if err != nil {
				return err
			}

			if future, ok := o.(inf.Future); ok {
				if err := ins.AddTask(future); err != nil {
					return err
				}
			}
			break
		case config.RoleCli:
			c := &model.Client{
				EndPoint: iConfig.EndPoint,
				Inbounds: iConfig.Inbounds,
			}

			o, err := common.GetServerInstance(ins.Ctx, c)
			if err != nil {
				return err
			}

			if future, ok := o.(inf.Future); ok {
				if err := ins.AddTask(future); err != nil {
					return err
				}
			}
			break
		}
	}
	return nil
}

type Instance struct {
	Lock    sync.Mutex
	Ctx     context.Context
	Cancel  context.CancelFunc
	Futures []inf.Future
	Running bool
}

func (i *Instance) AddTask(o inf.Future) error {
	i.Futures = append(i.Futures, o)
	if i.Running {
		if err := o.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (i *Instance) AddTasks(o []inf.Future) error {
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

	zlog.Warn("Endpoint started")

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
