package http

import (
	"context"
	"encoding/json"
	"golang.org/x/net/quic"
	"myproxy/internal/mlog"
	"myproxy/pkg/io"
	"myproxy/pkg/models"
	"myproxy/pkg/shared"
	"net"
)

func outboundHttp(ctx context.Context, buf []byte, client net.Conn, stream *quic.Stream) {
	i := models.InitialPacket{
		Protocol: shared.HTTP,
		Content:  buf,
	}

	m, err := json.Marshal(i)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	_, err = stream.Write(m)
	if err != nil {
		mlog.Error(err.Error())
		return
	}
	stream.Flush()

	p := io.Pipe{
		Stream: stream,
	}

	io.Copy(&p, client)
}
