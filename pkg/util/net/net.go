package net

import (
	"fmt"
	"github.com/spf13/viper"
	"io"
	"math/rand"
	"myproxy/internal/mlog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	address  string
	one      sync.Once
	highPort = 60000
	lowPort  = 10000
)

func GetExternalIP() (string, error) {
	if address == "" {
		resp, err := http.DefaultClient.Get("https://ipinfo.io/ip")
		if err != nil {
			return "", err
		}
		bytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		address = string(bytes)
	}
	return address, nil
}

func GetFreePort() uint16 {
	one.Do(func() {
		value := viper.GetString("endpoint.randPort")
		if value != "" {
			split := strings.Split(value, "-")
			low, err := strconv.Atoi(split[0])
			mlog.UnwrapFatal(err)
			high, err := strconv.Atoi(split[1])
			mlog.UnwrapFatal(err)

			if low < high {
				highPort = high
				lowPort = low
			}
		}
	})

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		p := r.Intn(highPort-lowPort) + lowPort
		if !CheckPortAvailability(p) {
			return uint16(p)
		}
	}
}

func CheckPortAvailability(port int) bool {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true
	}
	defer func() {
		if listen != nil {
			mlog.Unwrap(listen.Close())
		}
	}()

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	listenUDP, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return true
	}
	defer func() {
		if listenUDP != nil {
			mlog.Unwrap(listenUDP.Close())
		}
	}()

	return false
}

func GetTcpListener() (net.Listener, uint16, error) {
	port := GetFreePort()
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, 0, err
	}

	mlog.Info(fmt.Sprintf("listening TCP on [%s]%s", l.Addr().Network(), l.Addr().String()))

	return l, port, nil
}

func GetUdpListener() (*net.UDPConn, uint16, error) {
	port := GetFreePort()
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, 0, err
	}

	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, 0, err
	}

	mlog.Info(fmt.Sprintf("listening UDP in [%s]%s", l.LocalAddr().Network(), l.LocalAddr().String()))

	return l, port, nil
}
