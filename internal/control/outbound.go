package control

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/net/quic"
	"myproxy/internal"
	"myproxy/pkg/di"
	"myproxy/pkg/models"
	"myproxy/pkg/protocol"
	"myproxy/pkg/util/net"
	"myproxy/pkg/util/packet"
	"reflect"
	"sync"
)

type outboundServer struct {
	Ctx       context.Context
	Outbounds []*models.Outbound
}

func (o *outboundServer) Run() error {
	var wg sync.WaitGroup
	var errs []error
	for _, outbound := range o.Outbounds {
		wg.Add(1)
		go initial(o.Ctx, &wg, outbound, errs)
	}
	wg.Wait()
	err := errors.Join(errs...)
	if err != nil {
		return err
	}

	return nil
}

func (o *outboundServer) Close() error {
	return nil
}

func initial(ctx context.Context, wg *sync.WaitGroup, oub *models.Outbound, errs []error) {
	defer wg.Done()

	endpoint, err := protocol.GetEndpoint(&models.NetAddr{Port: net.GetFreePort()})
	if err != nil {
		errs = append(errs, err)
		return
	}
	defer func(endpoint *quic.Endpoint, ctx context.Context) {
		err := endpoint.Close(ctx)
		if err != nil {
			return
		}
	}(endpoint, ctx)

	dial, err := protocol.GetEndPointDial(ctx, endpoint, &models.NetAddr{Address: oub.Address, Port: oub.Port})
	if err != nil {
		errs = append(errs, err)
		return
	}
	defer func(dial *quic.Conn) {
		err := dial.Close()
		if err != nil {
			return
		}
	}(dial)

	stream, err := dial.NewStream(ctx)
	if err != nil {
		errs = append(errs, err)
		return
	}
	defer func(stream *quic.Stream) {
		err := stream.Close()
		if err != nil {
			return
		}
	}(stream)

	msg := internal.Message{
		Tag:      oub.Tag,
		NodePort: oub.NodePort,
	}

	m, err := json.Marshal(&msg)
	if err != nil {
		errs = append(errs, err)
		return
	}

	_, err = stream.Write(packet.EnPacket(m))
	if err != nil {
		errs = append(errs, err)
		return
	}
	stream.Flush()

	dePacket, err := packet.DePacket(stream)
	if err != nil {
		errs = append(errs, err)
		return
	}

	var newMsg internal.Message
	err = json.Unmarshal(dePacket, &newMsg)
	if err != nil {
		errs = append(errs, err)
		return
	}

	internal.Osi[oub.Tag] = internal.OutSeverInfo{
		Tag:      oub.Tag,
		Address:  oub.Address,
		NodePort: newMsg.NodePort,
	}
}

func outboundServerCreator(ctx context.Context, v any) (any, error) {
	outbounds := v.([]*models.Outbound)
	return &outboundServer{Ctx: ctx, Outbounds: outbounds}, nil
}

func init() {
	pc := reflect.TypeOf([]*models.Outbound{})
	di.ServerContext[pc] = outboundServerCreator
}
