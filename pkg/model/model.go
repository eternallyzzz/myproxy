package model

import (
	"fmt"
	"net"
	"time"
)

type Config struct {
	Log      *Log       `json:"log"`
	Quic     *Transfer  `json:"transfer"`
	Role     []string   `json:"role"`
	Filter   []string   `json:"filter"`
	Inbounds []*Service `json:"inbounds"`
	EndPoint *Endpoint  `json:"endpoint"`
}

type Log struct {
	ConsoleLevel string `json:"consoleLevel"`
	FileLevel    string `json:"fileLevel"`
	LogFilePath  string `json:"logFilePath"`
}

type Transfer struct {
	TLS *Tls `json:"tls"`
	QUICCfg
}

type Endpoint struct {
	RandPort string `json:"randPort"`
	*NetAddr
}

type QUICCfg struct {
	MaxBidiRemoteStreams     int64         `json:"maxBidiRemoteStreams"`
	MaxUniRemoteStreams      int64         `json:"maxUniRemoteStreams"`
	MaxStreamReadBufferSize  int64         `json:"maxStreamReadBufferSize"`
	MaxStreamWriteBufferSize int64         `json:"maxStreamWriteBufferSize"`
	MaxConnReadBufferSize    int64         `json:"maxConnReadBufferSize"`
	RequireAddressValidation bool          `json:"requireAddressValidation"`
	HandshakeTimeout         time.Duration `json:"handshakeTimeout"`
	MaxIdleTimeout           time.Duration `json:"maxIdleTimeout"`
	KeepAlivePeriod          time.Duration `json:"keepAlivePeriod"`
}

type Tls struct {
	Crt string `json:"crt"`
	Key string `json:"key"`
}

type NetAddr struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

func (n *NetAddr) String() string {
	return fmt.Sprintf("%s:%d", n.Address, n.Port)
}

type Service struct {
	Tag      string `json:"tag"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

func (s *Service) String() string {
	return fmt.Sprintf("%s:%d", s.Address, s.Port)
}

type Handshake struct {
	Tag     string
	Network string
}

type Client struct {
	EndPoint *Endpoint
	Inbounds []*Service
}

type InitialPacket struct {
	Protocol string  `json:"protocol"`
	Content  []byte  `json:"content"`
	Request  Request `json:"request"`
}

type Request struct {
	Network string `json:"network"`
	ID      string `json:"id"`
	Address string `json:"address"`
}

type Packet struct {
	Content []byte
	Addr    *net.UDPAddr
}
