package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/net/quic"
	"reflect"
	"testDemo/core/forward"
	"testDemo/pkg/common"
	net2 "testDemo/pkg/kit/net"
	"testDemo/pkg/model"
	"testDemo/pkg/zlog"
	"time"
)

type Listen struct {
	Ctx      context.Context
	Endpoint *quic.Endpoint
}

func (l *Listen) Run() error {
	zlog.Warn(fmt.Sprintf("Control listening UDP stream on %s", l.Endpoint.LocalAddr().String()))
	go func() {
		for {
			conn, err := l.Endpoint.Accept(l.Ctx)
			if err != nil {
				select {
				case <-l.Ctx.Done():
					return
				default:
					zlog.Error("failed accept quic", zap.Error(err))
					time.Sleep(time.Second)
					continue
				}
			}
			go handleInitial(l.Ctx, conn)
		}
	}()
	return nil
}

func (l *Listen) Close() error {
	err := l.Endpoint.Close(l.Ctx)
	if err != nil {
		return err
	}

	return nil
}

func handleInitial(ctx context.Context, conn *quic.Conn) {
	defer conn.Close()

	port := net2.GetFreePort()
	ip, err := net2.GetExternalIP()
	if err != nil {
		zlog.Error(err.Error())
		return
	}

	addr := model.NetAddr{
		Address: ip,
		Port:    port,
	}

	m, err := json.Marshal(addr)
	if err != nil {
		zlog.Error(err.Error())
		return
	}

	sendOnlyStream, err := conn.NewSendOnlyStream(ctx)
	if err != nil {
		zlog.Error(err.Error())
		return
	}
	defer sendOnlyStream.Close()

	laddr := model.NetAddr{Port: port}

	endpoint, err := common.GetEndpoint(&laddr)
	if err != nil {
		zlog.Error(err.Error())
		return
	}
	zlog.Info(fmt.Sprintf("listening UDP stream on %s", laddr.String()))

	go forward.ListenQUIC(ctx, endpoint)

	_, err = sendOnlyStream.Write(m)
	if err != nil {
		endpoint.Close(ctx)
		zlog.Error(err.Error())
		return
	}
	sendOnlyStream.Flush()

	time.Sleep(time.Second * 5)
}

type Client struct {
	Ctx       context.Context
	ClientCfg *model.Client
	Endpoint  *quic.Endpoint
}

func (c *Client) Run() error {
	nd := &model.NetAddr{
		Port: net2.GetFreePort(),
	}

	endpoint, err := common.GetEndpoint(nd)
	if err != nil {
		return err
	}
	dial, err := common.GetEndPointDial(c.Ctx, endpoint, c.ClientCfg.EndPoint.NetAddr)
	if err != nil {
		return err
	}

	buff := make([]byte, 1500)

	for {
		readStream, err := dial.AcceptStream(c.Ctx)
		if err != nil {
			return err
		}

		n, err := readStream.Read(buff)
		if err != nil {
			return err
		}

		var nAddr *model.NetAddr

		err = json.Unmarshal(buff[:n], &nAddr)
		if err != nil {
			return err
		}

		ip, err := net2.GetExternalIP()
		if err != nil {
			return err
		}

		if nAddr.Address == ip {
			nAddr.Address = "127.0.0.1"
		}

		getEndpoint, err := common.GetEndpoint(&model.NetAddr{Port: net2.GetFreePort()})
		if err != nil {
			return err
		}
		c.Endpoint = getEndpoint

		pointDial, err := common.GetEndPointDial(c.Ctx, getEndpoint, nAddr)
		if err != nil {
			return err
		}

		go forward.Process(c.Ctx, pointDial, c.ClientCfg.Inbounds)

		break
	}
	return nil
}

func (c *Client) Close() error {
	err := c.Endpoint.Close(c.Ctx)
	if err != nil {
		return err
	}

	return nil
}

func ListenCreator(ctx context.Context, v any) (any, error) {
	listenConfig, ok := v.(*model.Endpoint)
	if !ok {
		return nil, errors.New("invalid config type")
	}

	endpoint, err := common.GetEndpoint(listenConfig.NetAddr)
	if err != nil {
		return nil, err
	}

	return &Listen{Ctx: ctx, Endpoint: endpoint}, nil
}

func ClientCreator(ctx context.Context, v any) (any, error) {
	client := v.(*model.Client)

	return &Client{Ctx: ctx, ClientCfg: client}, nil
}

func init() {
	lc := reflect.TypeOf(&model.Endpoint{})
	common.ServerContext[lc] = ListenCreator
	cc := reflect.TypeOf(&model.Client{})
	common.ServerContext[cc] = ClientCreator
}
