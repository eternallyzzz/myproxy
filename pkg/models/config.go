package models

import (
	"fmt"
	"net"
	"time"
)

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

type Config struct {
	Log       *Log        `json:"log"`
	Transfer  *Transfer   `json:"transfer"`
	Inbounds  []*Inbound  `json:"inbounds"`
	Outbounds []*Outbound `json:"outbounds"`
	Endpoint  *Endpoint   `json:"endpoint"`
	Routing   *Routing    `json:"routing"`
}

type Log struct {
	ConsoleLevel string `json:"consoleLevel"`
	FileLevel    string `json:"fileLevel"`
	LogFilePath  string `json:"logFilePath"`
}

type Transfer struct {
	TLS *Tls `json:"tls"`
	*QUICConfig
}

type Endpoint struct {
	RandPort string `json:"randPort"`
	*NetAddr
}

type QUICConfig struct {
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
	Port    uint16 `json:"port"`
}

func (n *NetAddr) String() string {
	return fmt.Sprintf("%s:%d", n.Address, n.Port)
}

type Inbound struct {
	Tag      string   `json:"tag"`
	Address  string   `json:"address"`
	Port     uint16   `json:"port"`
	Protocol string   `json:"protocol"`
	Setting  *Setting `json:"setting"`
}

func (i *Inbound) AddrPort() string {
	return fmt.Sprintf("%s:%d", i.Address, i.Port)
}

type Setting struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type Outbound struct {
	Tag      string `json:"tag"`
	Address  string `json:"address"`
	Port     uint16 `json:"port"`
	NodePort uint16 `json:"nodePort"`
}

type Routing struct {
	Rules []*Rule `json:"rules"`
}

type Rule struct {
	InTag  string   `json:"inTag"`
	OutTag string   `json:"outTag"`
	IP     []string `json:"ip"`
}
