package http

import (
	"context"
	"encoding/json"
	"golang.org/x/net/quic"
	"net"
	"testDemo/pkg/common"
	"testDemo/pkg/config"
	"testDemo/pkg/model"
	"testDemo/pkg/zlog"
)

func outboundHttp(ctx context.Context, buf []byte, client net.Conn, quicConn *quic.Conn) {
	stream, err := quicConn.NewStream(ctx)
	if err != nil {
		zlog.Error(err.Error())
		return
	}

	i := model.InitialPacket{
		Protocol: config.HTTP,
		Content:  buf,
	}

	m, err := json.Marshal(i)
	if err != nil {
		zlog.Error(err.Error())
		return
	}

	_, err = stream.Write(m)
	if err != nil {
		zlog.Error(err.Error())
		return
	}
	stream.Flush()

	var buff [64]byte
	_, err = stream.Read(buff[:])
	if err != nil {
		zlog.Error(err.Error())
		return
	}

	p := common.Pipe{
		Stream: stream,
	}

	common.Copy(&p, client)
}
