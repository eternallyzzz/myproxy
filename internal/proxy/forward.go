package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/net/quic"
	"myproxy/internal/mlog"
	"myproxy/internal/proxy/http"
	"myproxy/internal/proxy/socks"
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
	defer func(conn *quic.Conn) {
		err := conn.Close()
		if err != nil {
			return
		}
	}(conn)

	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
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
			go http.Process(ctx, i.Content, stream)
			break
		case shared.SOCKS:
			go socks.Process(ctx, i.Request, stream)
			break
		}
	}
}

func Process(ctx context.Context, inb *models.Inbound) {
	switch inb.Protocol {
	case shared.SOCKS:
		go socks.Inbound(ctx, inb)
		break
	case shared.HTTP:
		go http.Inbound(ctx, inb)
		break
	}
}
