package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/net/quic"
	"io"
	"myproxy/internal"
	"myproxy/internal/mlog"
	"myproxy/internal/router"
	io2 "myproxy/pkg/io"
	"myproxy/pkg/models"
	"myproxy/pkg/protocol"
	"myproxy/pkg/shared"
	net2 "myproxy/pkg/util/net"
	"net"
	"net/http"
	"strings"
)

func Process(ctx context.Context, payload []byte, stream *quic.Stream) {
	mlog.Debug(string(payload))
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(payload)))
	if err != nil {
		mlog.Error("Failed to parse client request:", zap.Error(err))
		return
	}

	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		mlog.Error("Failed to parse target host:", zap.Error(err))
		return
	}

	ips, err := net2.LookupIP(host)
	if err != nil {
		mlog.Error("Failed to resolve target host:", zap.Error(err))
		return
	}
	if len(ips) == 0 {
		mlog.Error("no IPs resolved for " + host)
		return
	}

	r := router.Router{DstAddr: ips[0]}
	outTag := r.Process()

	if outTag == "direct" {
		mlog.Debug(fmt.Sprintf("request to Method [%s] Host [%s] with URL [%s]", req.Method, host, req.URL))

		p := io2.Pipe{
			Stream: stream,
		}

		handleClientRequest(payload, req, &p)
	} else {
		info, ok := internal.GetOsi(outTag)
		if !ok {
			mlog.Error("outbound not found: " + outTag)
			return
		}
		remoteAddr := &models.NetAddr{Address: info.Address, Port: info.NodePort}

		newStream, err := protocol.StreamPool(ctx, remoteAddr)
		if err != nil {
			mlog.Error(err.Error())
			return
		}

		i := models.InitialPacket{
			Protocol: shared.HTTP,
			Content:  payload,
		}

		m, err := json.Marshal(i)
		if err != nil {
			mlog.Error(err.Error())
			return
		}

		_, err = newStream.Write(m)
		if err != nil {
			mlog.Error(err.Error())
			return
		}
		newStream.Flush()

		input := io2.Pipe{Stream: stream}
		output := io2.Pipe{Stream: newStream}

		io2.Copy(&output, &input)
	}
}

func handleConnectRequest(client io.ReadWriteCloser, targetHost string, targetPort string) {
	targetConn, err := net.Dial("tcp", targetHost+":"+targetPort)
	if err != nil {
		mlog.Error("Failed to connect to target:", zap.Error(err))
		err := client.Close()
		if err != nil {
			return
		}
		return
	}
	defer func(targetConn net.Conn) {
		err := targetConn.Close()
		if err != nil {
			return
		}
	}(targetConn)

	mlog.Debug(fmt.Sprintf("connection opened to tcp:%s, local endpoint %s, remote endpoint %s",
		targetHost+":"+targetPort, targetConn.LocalAddr(), targetConn.LocalAddr()))
	_, err = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		return
	}

	io2.Copy(targetConn, client)
}

func handleHTTPRequest(client io.ReadWriteCloser, targetHost string, targetPort string, requestData []byte) {
	targetConn, err := net.Dial("tcp", targetHost+":"+targetPort)
	if err != nil {
		mlog.Error("Failed to connect to target:", zap.Error(err))
		err := client.Close()
		if err != nil {
			return
		}
		return
	}
	defer func(targetConn net.Conn) {
		err := targetConn.Close()
		if err != nil {
			return
		}
	}(targetConn)
	mlog.Debug(fmt.Sprintf("connection opened to tcp:%s, local endpoint %s, remote endpoint %s",
		targetHost+":"+targetPort, targetConn.LocalAddr(), targetConn.LocalAddr()))

	_, err = targetConn.Write(requestData)
	if err != nil {
		return
	}

	io2.Copy(targetConn, client)
}

func handleClientRequest(buf []byte, req *http.Request, client io.ReadWriteCloser) {
	if req.Method == "CONNECT" {
		targetHost, targetPort, err := net.SplitHostPort(req.Host)
		if err != nil {
			mlog.Error("Failed to parse target host:", zap.Error(err))
			return
		}
		handleConnectRequest(client, targetHost, targetPort)
	} else {
		targetHost, targetPort, err := net.SplitHostPort(req.Host)
		if err != nil {
			if !strings.Contains(err.Error(), "missing port in address") {
				mlog.Error(fmt.Sprintf("Failed to parse target host:%T", err), zap.Error(err))
				return
			}
			targetHost = req.Host
			targetPort = "80"
		}
		handleHTTPRequest(client, targetHost, targetPort, buf)
	}
}
