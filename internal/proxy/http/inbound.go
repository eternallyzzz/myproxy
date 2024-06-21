package http

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go.uber.org/zap"
	"myproxy/internal/mlog"
	"myproxy/pkg/models"
	"myproxy/pkg/shared"
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

	mlog.Debug("listening TCP on " + l.Addr().String())

	for {
		accept, err := l.Accept()
		if err != nil {
			mlog.Error(err.Error())
			return
		}

		mlog.Debug("accepted TCP connection " + accept.RemoteAddr().String())

		go dispatchHttp(ctx, accept)
	}
}

func dispatchHttp(ctx context.Context, client net.Conn) {
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

	if shared.IPDB != nil {
		country, err := shared.IPDB.Country(ip[0])
		if err != nil {
			mlog.Error(err.Error())
			return
		}

		for _, filter := range shared.Filters {
			if country.Country.IsoCode == filter {
				mlog.Debug(fmt.Sprintf("request %s with [direct]", req.URL))
				handleClientRequest(buf[:n], req, client)
				return
			}
		}
	}

	mlog.Debug(fmt.Sprintf("request %s with [%s]", req.URL, conn.String()))
	outboundHttp(ctx, buf[:n], client, conn)
}
