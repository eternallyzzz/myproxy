package http

import (
	"bufio"
	"bytes"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/net/quic"
	"io"
	"net"
	"net/http"
	"strings"
	"testDemo/pkg/common"
	"testDemo/pkg/zlog"
)

func Process(payload []byte, stream *quic.Stream) {
	zlog.Debug(string(payload))
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(payload)))
	if err != nil {
		zlog.Error("Failed to parse client request:", zap.Error(err))
		return
	}

	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		zlog.Error("Failed to parse target host:", zap.Error(err))
		return
	}

	_, err = stream.Write([]byte("ok"))
	if err != nil {
		zlog.Error(err.Error())
		return
	}
	stream.Flush()

	zlog.Debug(fmt.Sprintf("request to Method [%s] Host [%s] with URL [%s]", req.Method, host, req.URL))

	p := common.Pipe{
		Stream: stream,
	}

	handleClientRequest(payload, req, &p)
}

func handleConnectRequest(client io.ReadWriteCloser, targetHost string, targetPort string) {
	// 1. 建立到目标服务器的连接
	targetConn, err := net.Dial("tcp", targetHost+":"+targetPort)
	if err != nil {
		zlog.Error("Failed to connect to target:", zap.Error(err))
		client.Close()
		return
	}
	defer targetConn.Close()

	zlog.Debug(fmt.Sprintf("connection opened to tcp:%s, local endpoint %s, remote endpoint %s",
		targetHost+":"+targetPort, targetConn.LocalAddr(), targetConn.LocalAddr()))
	// 2. 向客户端发送成功响应
	client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// 3. 数据转发
	common.Copy(targetConn, client)
}

func handleHTTPRequest(client io.ReadWriteCloser, targetHost string, targetPort string, requestData []byte) {
	// 1. 建立到目标服务器的连接
	targetConn, err := net.Dial("tcp", targetHost+":"+targetPort)
	if err != nil {
		zlog.Error("Failed to connect to target:", zap.Error(err))
		client.Close()
		return
	}
	defer targetConn.Close()
	zlog.Debug(fmt.Sprintf("connection opened to tcp:%s, local endpoint %s, remote endpoint %s",
		targetHost+":"+targetPort, targetConn.LocalAddr(), targetConn.LocalAddr()))

	// 2. 将 HTTP 请求发送给目标服务器
	targetConn.Write(requestData)

	// 3. 数据转发
	common.Copy(targetConn, client)
}

func handleClientRequest(buf []byte, req *http.Request, client io.ReadWriteCloser) {
	// 处理 CONNECT 请求
	if req.Method == "CONNECT" {
		targetHost, targetPort, err := net.SplitHostPort(req.Host)
		if err != nil {
			zlog.Error("Failed to parse target host:", zap.Error(err))
			return
		}
		handleConnectRequest(client, targetHost, targetPort)
	} else {
		// 提取目标主机和端口
		targetHost, targetPort, err := net.SplitHostPort(req.Host)
		if err != nil {
			if !strings.Contains(err.Error(), "missing port in address") {
				zlog.Error(fmt.Sprintf("Failed to parse target host:%T", err), zap.Error(err))
				return
			}
			targetHost = req.Host
			targetPort = "80"
		}
		// 处理 HTTP 请求
		handleHTTPRequest(client, targetHost, targetPort, buf)
	}
}
