package proxy

import (
	"context"
	"encoding/json"
	"golang.org/x/net/quic"
	"myproxy/internal/mlog"
	http2 "myproxy/internal/proxy/http"
	socks2 "myproxy/internal/proxy/socks"
	"myproxy/pkg/models"
	"myproxy/pkg/shared"
)

func ListenQUIC(ctx context.Context, l *quic.Endpoint) {
	for {
		accept, err := l.Accept(ctx)
		if err != nil {
			mlog.Error(err.Error())
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
			mlog.Error(err.Error())
			return
		}

		var buf [1500]byte

		n, err := stream.Read(buf[:])
		if err != nil {
			mlog.Error(err.Error())
			return
		}

		var i models.InitialPacket
		err = json.Unmarshal(buf[:n], &i)
		if err != nil {
			mlog.Error(err.Error())
			return
		}

		switch i.Protocol {
		case shared.HTTP:
			go http2.Process(i.Content, stream)
			break
		case shared.SOCKS:
			go socks2.Process(i.Request, stream)
			break
		}
	}
}

func Process(ctx context.Context, conn *quic.Conn, inbs []*models.Service) {
	for _, inbound := range inbs {
		switch inbound.Protocol {
		case shared.SOCKS:
			go socks2.Inbound(ctx, inbound, conn)
			break
		case shared.HTTP:
			go http2.Inbound(ctx, inbound, conn)
			break
		}
	}
}
