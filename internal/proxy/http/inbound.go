package http

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go.uber.org/zap"
	"myproxy/internal"
	"myproxy/internal/mlog"
	"myproxy/internal/router"
	"myproxy/pkg/models"
	"myproxy/pkg/protocol"
	net2 "myproxy/pkg/util/net"
	"net"
	"net/http"
)

func Inbound(ctx context.Context, inb *models.Inbound) {
	l, err := net.Listen("tcp", inb.AddrPort())
	if err != nil {
		mlog.Error(err.Error())
		return
	}
	defer l.Close()

	mlog.Info("listening TCP on " + l.Addr().String())

	for {
		accept, err := l.Accept()
		if err != nil {
			mlog.Error(err.Error())
			return
		}

		mlog.Debug("accepted TCP connection " + accept.RemoteAddr().String())

		go dispatchHttp(ctx, accept, inb)
	}
}

func dispatchHttp(ctx context.Context, client net.Conn, inb *models.Inbound) {
	var buf [4096]byte
	n, err := client.Read(buf[:])
	if err != nil {
		mlog.Error("Failed to read client request:", zap.Error(err))
		return
	}

	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(buf[:n])))
	if err != nil {
		mlog.Error("Failed to parse client request:", zap.Error(err))
		return
	}

	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		mlog.Error("Failed to parse target host:", zap.Error(err))
		return
	}

	mlog.Debug(fmt.Sprintf("request to Method [%s] Host [%s] with URL [%s]", req.Method, host, req.URL))

	ip, err := net.LookupIP(host)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	r := router.Router{
		InboundTag: inb.Tag,
		DstAddr:    ip[0],
	}

	outTag := r.Process()

	if outTag == "direct" {
		mlog.Debug(fmt.Sprintf("request %s with [direct]", req.URL))
		handleClientRequest(buf[:n], req, client)
		return
	}

	info := internal.Osi[outTag]

	endpoint, err := protocol.GetEndpoint(&models.NetAddr{Port: net2.GetFreePort()})
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	dial, err := protocol.GetEndPointDial(ctx, endpoint, &models.NetAddr{Address: info.Address, Port: info.NodePort})
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	mlog.Debug(fmt.Sprintf("request %s with [%s]", req.URL, dial.String()))
	outboundHttp(ctx, buf[:n], client, dial)
}
