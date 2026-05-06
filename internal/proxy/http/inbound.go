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
	defer func(l net.Listener) {
		err := l.Close()
		if err != nil {
			return
		}
	}(l)

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
	var buf [65536]byte
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

	if inb.Setting != nil && inb.Setting.User != "" && inb.Setting.Pass != "" {
		u, p, ok := req.BasicAuth()
		if !ok || u != inb.Setting.User || p != inb.Setting.Pass {
			_, _ = client.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic\r\n\r\n"))
			_ = client.Close()
			return
		}
	}

	ips, err := net2.LookupIP(host)
	if err != nil {
		mlog.Error(err.Error())
		return
	}
	if len(ips) == 0 {
		mlog.Error("no IPs resolved for " + host)
		return
	}

	r := router.Router{
		InboundTag: inb.Tag,
		DstAddr:    ips[0],
	}

	outTag := r.Process()

	if outTag == "direct" {
		mlog.Debug(fmt.Sprintf("request %s with [direct]", req.URL))
		handleClientRequest(buf[:n], req, client)
		return
	}

	info, ok := internal.GetOsi(outTag)
	if !ok {
		mlog.Error("outbound not found: " + outTag)
		return
	}
	remoteAddr := &models.NetAddr{Address: info.Address, Port: info.NodePort}

	stream, err := protocol.StreamPool(ctx, remoteAddr)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	mlog.Debug(fmt.Sprintf("request %s with [%s]", req.URL, remoteAddr.String()))
	outboundHttp(ctx, buf[:n], client, stream)
}
