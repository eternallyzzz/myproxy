package http

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/net/quic"
	"net"
	"net/http"
	"testDemo/pkg/config"
	"testDemo/pkg/model"
	"testDemo/pkg/zlog"
)

func Inbound(ctx context.Context, s *model.Service, conn *quic.Conn) {
	l, err := net.Listen("tcp", s.String())
	if err != nil {
		zlog.Error(err.Error())
		return
	}
	defer l.Close()

	zlog.Debug("listening TCP on " + l.Addr().String())

	for {
		accept, err := l.Accept()
		if err != nil {
			zlog.Error(err.Error())
			return
		}

		zlog.Debug("accepted TCP connection " + accept.RemoteAddr().String())

		go dispatchHttp(ctx, accept, conn)
	}
}

func dispatchHttp(ctx context.Context, client net.Conn, conn *quic.Conn) {
	var buf [4096]byte
	n, err := client.Read(buf[:])
	if err != nil {
		zlog.Error("Failed to read client request:", zap.Error(err))
		return
	}

	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(buf[:n])))
	if err != nil {
		zlog.Error("Failed to parse client request:", zap.Error(err))
		return
	}

	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		zlog.Error("Failed to parse target host:", zap.Error(err))
		return
	}

	zlog.Debug(fmt.Sprintf("request to Method [%s] Host [%s] with URL [%s]", req.Method, host, req.URL))

	ip, err := net.LookupIP(host)
	if err != nil {
		zlog.Error(err.Error())
		return
	}

	if config.IPDB != nil {
		country, err := config.IPDB.Country(ip[0])
		if err != nil {
			zlog.Error(err.Error())
			return
		}

		for _, filter := range config.Filters {
			if country.Country.IsoCode == filter {
				zlog.Debug(fmt.Sprintf("request %s with [direct]", req.URL))
				handleClientRequest(buf[:n], req, client)
				return
			}
		}
	}

	zlog.Debug(fmt.Sprintf("request %s with [%s]", req.URL, conn.String()))
	outboundHttp(ctx, buf[:n], client, conn)
}
