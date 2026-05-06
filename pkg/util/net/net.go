package net

import (
	"fmt"
	"io"
	"myproxy/internal/mlog"
	"net"
	"net/http"
	"strings"
)

var (
	address string
)

func GetExternalIP() (string, error) {
	if address == "" {
		resp, err := http.DefaultClient.Get("https://ipinfo.io/ip")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		bytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		address = strings.TrimSpace(string(bytes))
	}
	return address, nil
}

func GetFreePort() uint16 {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0
	}
	defer l.Close()
	return uint16(l.Addr().(*net.TCPAddr).Port)
}

func GetTcpListener() (net.Listener, uint16, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, 0, err
	}

	mlog.Info(fmt.Sprintf("listening TCP on [%s]%s", l.Addr().Network(), l.Addr().String()))

	return l, uint16(l.Addr().(*net.TCPAddr).Port), nil
}

func GetUdpListener() (*net.UDPConn, uint16, error) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return nil, 0, err
	}

	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, 0, err
	}

	mlog.Info(fmt.Sprintf("listening UDP in [%s]%s", l.LocalAddr().Network(), l.LocalAddr().String()))

	return l, uint16(l.LocalAddr().(*net.UDPAddr).Port), nil
}
