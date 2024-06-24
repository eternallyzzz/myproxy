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

	ip, err := net.LookupIP(host)
	if err != nil {
		mlog.Error("Failed to resolve target host:", zap.Error(err))
		return
	}

	r := router.Router{DstAddr: ip[0]}
	outTag := r.Process()

	if outTag == "direct" {
		_, err = stream.Write([]byte("ok"))
		if err != nil {
			mlog.Error(err.Error())
			return
		}
		stream.Flush()

		mlog.Debug(fmt.Sprintf("request to Method [%s] Host [%s] with URL [%s]", req.Method, host, req.URL))

		p := io2.Pipe{
			Stream: stream,
		}

		handleClientRequest(payload, req, &p)
	} else {
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

		newStream, err := dial.NewStream(ctx)
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
	// 1. 建立到目标服务器的连接
	targetConn, err := net.Dial("tcp", targetHost+":"+targetPort)
	if err != nil {
		mlog.Error("Failed to connect to target:", zap.Error(err))
		client.Close()
		return
	}
	defer targetConn.Close()

	mlog.Debug(fmt.Sprintf("connection opened to tcp:%s, local endpoint %s, remote endpoint %s",
		targetHost+":"+targetPort, targetConn.LocalAddr(), targetConn.LocalAddr()))
	// 2. 向客户端发送成功响应
	client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// 3. 数据转发
	io2.Copy(targetConn, client)
}

func handleHTTPRequest(client io.ReadWriteCloser, targetHost string, targetPort string, requestData []byte) {
	// 1. 建立到目标服务器的连接
	targetConn, err := net.Dial("tcp", targetHost+":"+targetPort)
	if err != nil {
		mlog.Error("Failed to connect to target:", zap.Error(err))
		client.Close()
		return
	}
	defer targetConn.Close()
	mlog.Debug(fmt.Sprintf("connection opened to tcp:%s, local endpoint %s, remote endpoint %s",
		targetHost+":"+targetPort, targetConn.LocalAddr(), targetConn.LocalAddr()))

	// 2. 将 HTTP 请求发送给目标服务器
	targetConn.Write(requestData)

	// 3. 数据转发
	io2.Copy(targetConn, client)
}

func handleClientRequest(buf []byte, req *http.Request, client io.ReadWriteCloser) {
	// 处理 CONNECT 请求
	if req.Method == "CONNECT" {
		targetHost, targetPort, err := net.SplitHostPort(req.Host)
		if err != nil {
			mlog.Error("Failed to parse target host:", zap.Error(err))
			return
		}
		handleConnectRequest(client, targetHost, targetPort)
	} else {
		// 提取目标主机和端口
		targetHost, targetPort, err := net.SplitHostPort(req.Host)
		if err != nil {
			if !strings.Contains(err.Error(), "missing port in address") {
				mlog.Error(fmt.Sprintf("Failed to parse target host:%T", err), zap.Error(err))
				return
			}
			targetHost = req.Host
			targetPort = "80"
		}
		// 处理 HTTP 请求
		handleHTTPRequest(client, targetHost, targetPort, buf)
	}
}
