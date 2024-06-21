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

func outboundHttp(ctx context.Context, buf []byte, client net.Conn, quicConn *quic.Conn) {
	stream, err := quicConn.NewStream(ctx)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

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

	var buff [64]byte
	_, err = stream.Read(buff[:])
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	p := io.Pipe{
		Stream: stream,
	}

	io.Copy(&p, client)
}
