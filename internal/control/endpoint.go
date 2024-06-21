package control

import (
	"context"
	"encoding/json"
	"errors"
	"go.uber.org/zap"
	"golang.org/x/net/quic"
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

type server struct {
	Ctx       context.Context
	ServerCfg *models.Endpoint
	Endpoint  *quic.Endpoint
}

func (s *server) Run() error {
	endpoint, err := protocol.GetEndpoint(s.ServerCfg.NetAddr)
	if err != nil {
		return err
	}

	go listen(s.Ctx, endpoint)

	return nil
}

func (s *server) Close() error {
	err := s.Endpoint.Close(s.Ctx)
	if err != nil {
		return err
	}

	return nil
}

func listen(ctx context.Context, endpoint *quic.Endpoint) {
	defer endpoint.Close(ctx)

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
	defer conn.Close()

	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			mlog.Error("", zap.Error(err))
			return
		}

		go handStream(ctx, stream)
	}
}

func handStream(ctx context.Context, stream *quic.Stream) {
	defer stream.Close()

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

type Message struct {
	Tag      string `json:"tag"`
	NodePort uint16 `json:"nodePort"`
}

func decodePacket(payload []byte) *Message {
	var msg Message
	err := json.Unmarshal(payload, &msg)
	if err != nil {
		mlog.Error("", zap.Error(err))
		return nil
	}

	return &msg
}

func encodePacket(msg *Message, port uint16) []byte {
	message := Message{
		Tag:      msg.Tag,
		NodePort: port,
	}

	m, err := json.Marshal(message)
	if err != nil {
		mlog.Error("", zap.Error(err))
		return nil
	}

	return m
}

func getEndpoint(msg *Message) (*quic.Endpoint, error) {

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

func serverCreator(ctx context.Context, v any) (any, error) {
	listenConfig, ok := v.(*models.Endpoint)
	if !ok {
		return nil, errors.New("invalid config type")
	}

	return &server{Ctx: ctx, ServerCfg: listenConfig}, nil
}

func init() {
	sc := reflect.TypeOf(&models.Endpoint{})
	di.ServerContext[sc] = serverCreator
}
