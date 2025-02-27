package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/net/quic"
	"myproxy/internal"
	"myproxy/internal/mlog"
	"myproxy/internal/proxy"
	"myproxy/pkg/di"
	"myproxy/pkg/models"
	"myproxy/pkg/protocol"
	"myproxy/pkg/util/net"
	"myproxy/pkg/util/packet"
	"reflect"
	"time"
)

type endpointServer struct {
	Ctx       context.Context
	ServerCfg *models.Endpoint
	Endpoint  *quic.Endpoint
}

func (e *endpointServer) Run() error {
	endpoint, err := protocol.GetEndpoint(e.ServerCfg.NetAddr)
	if err != nil {
		return err
	}
	e.Endpoint = endpoint

	mlog.Warn(fmt.Sprintf("endpoint listen on %s", e.ServerCfg.NetAddr.String()))

	go listen(e.Ctx, endpoint)

	return nil
}

func (e *endpointServer) Close() error {
	err := e.Endpoint.Close(e.Ctx)
	if err != nil {
		return err
	}

	return nil
}

func listen(ctx context.Context, endpoint *quic.Endpoint) {
	for {
		accept, err := endpoint.Accept(ctx)
		if err != nil {
			mlog.Error("", zap.Error(err))
			return
		}

		go handConn(ctx, accept)
	}
}

func handConn(ctx context.Context, conn *quic.Conn) {
	defer func(conn *quic.Conn) {
		err := conn.Close()
		if err != nil {
			return
		}
	}(conn)

	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			if err.Error() == errConnClosed {
				return
			}
			mlog.Error("", zap.Error(err))
			return
		}

		go handStream(ctx, stream)
	}
}

func handStream(ctx context.Context, stream *quic.Stream) {
	defer func(stream *quic.Stream) {
		err := stream.Close()
		if err != nil {
			return
		}
	}(stream)

	for {
		payload, err := packet.DePacket(stream)
		if err != nil {
			mlog.Error("", zap.Error(err))
			return
		}

		message := decodePacket(payload)

		endpoint, err := getEndpoint(message)
		if err != nil {
			mlog.Error("", zap.Error(err))
			return
		}

		m := encodePacket(message, endpoint.LocalAddr().Port())

		_, err = stream.Write(m)
		if err != nil {
			mlog.Error("", zap.Error(err))
			return
		}
		stream.Flush()

		go proxy.ListenQUIC(ctx, endpoint)
		break
	}

	time.Sleep(time.Second * 5)
}

func decodePacket(payload []byte) *internal.Message {
	var msg internal.Message
	err := json.Unmarshal(payload, &msg)
	if err != nil {
		mlog.Error("", zap.Error(err))
		return nil
	}

	return &msg
}

func encodePacket(msg *internal.Message, port uint16) []byte {
	message := internal.Message{
		Tag:      msg.Tag,
		NodePort: port,
	}

	m, err := json.Marshal(message)
	if err != nil {
		mlog.Error("", zap.Error(err))
		return nil
	}

	return packet.EnPacket(m)
}

func getEndpoint(msg *internal.Message) (*quic.Endpoint, error) {

	var nd *models.NetAddr

	if msg.NodePort == 0 {
		nd = &models.NetAddr{Port: net.GetFreePort()}
	} else if msg.NodePort > 0 {
		nd = &models.NetAddr{Port: msg.NodePort}
	}

	endpoint, err := protocol.GetEndpoint(nd)
	if err != nil {
		return nil, err
	}

	return endpoint, nil
}

func endpointServerCreator(ctx context.Context, v any) (any, error) {
	listenConfig, ok := v.(*models.Endpoint)
	if !ok {
		return nil, errors.New("invalid config type")
	}

	return &endpointServer{Ctx: ctx, ServerCfg: listenConfig}, nil
}

func init() {
	sc := reflect.TypeOf(&models.Endpoint{})
	di.ServerContext[sc] = endpointServerCreator
}

var (
	errConnClosed = "connection closed"
)
