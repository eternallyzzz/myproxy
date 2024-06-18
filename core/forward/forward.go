package forward

import (
	"context"
	"encoding/json"
	"golang.org/x/net/quic"
	"testDemo/core/forward/http"
	"testDemo/core/forward/socks"
	"testDemo/pkg/config"
	"testDemo/pkg/model"
	"testDemo/pkg/zlog"
)

func ListenQUIC(ctx context.Context, l *quic.Endpoint) {
	for {
		accept, err := l.Accept(ctx)
		if err != nil {
			zlog.Error(err.Error())
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
			zlog.Error(err.Error())
			return
		}

		var buf [1500]byte

		n, err := stream.Read(buf[:])
		if err != nil {
			zlog.Error(err.Error())
			return
		}

		var i model.InitialPacket
		err = json.Unmarshal(buf[:n], &i)
		if err != nil {
			zlog.Error(err.Error())
			return
		}

		switch i.Protocol {
		case config.HTTP:
			go http.Process(i.Content, stream)
			break
		case config.SOCKS:
			go socks.Process(i.Request, stream)
			break
		}
	}
}

func Process(ctx context.Context, conn *quic.Conn, inbs []*model.Service) {
	for _, inbound := range inbs {
		switch inbound.Protocol {
		case config.SOCKS:
			go socks.Inbound(ctx, inbound, conn)
			break
		case config.HTTP:
			go http.Inbound(ctx, inbound, conn)
			break
		}
	}
}
