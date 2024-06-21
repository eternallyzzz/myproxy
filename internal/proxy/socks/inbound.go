package socks

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/protocol/socks/socks5"
	"go.uber.org/zap"
	"golang.org/x/net/quic"
	"io"
	"log"
	"myproxy/internal/mlog"
	io2 "myproxy/pkg/io"
	"myproxy/pkg/models"
	"myproxy/pkg/shared"
	"myproxy/pkg/util/id"
	"net"
	"net/netip"
	"strconv"
	"sync"
)

func Inbound(ctx context.Context, s *models.Service, conn *quic.Conn) {
	udpAddr, err := net.ResolveUDPAddr("udp", s.String())

	l, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		mlog.Error(err.Error())
		return
	}
	defer l.Close()

	mlog.Info("listening UDP on " + udpAddr.String())

	go listenUDP(ctx, l, conn)

	tl, err := net.Listen("tcp", s.String())
	if err != nil {
		log.Fatal("Failed to start listener:", err)
	}
	mlog.Info("listening TCP on " + tl.Addr().String())

	for {
		client, err := tl.Accept()
		if err != nil {
			mlog.Error("Failed to accept client connection:", zap.Error(err))
			return
		}

		go tcp(ctx, client, udpAddr, conn)
	}
}

func listenUDP(ctx context.Context, l *net.UDPConn, quicConn *quic.Conn) {
	defer l.Close()

	buff := make([]byte, 1500)

	for {
		n, addr, err := l.ReadFromUDP(buff)
		if err != nil {
			mlog.Error(err.Error())
			return
		}
		mlog.Debug("client connection from " + addr.String())

		data := buff[:n]

		value, ok := hm.Load(addr.Network() + addr.String())
		if ok {
			work := value.(*Work)
			work.Input <- data
			continue
		}

		if data[0] == 0 && data[1] == 0 {
			addrOffset := 4
			portOffset := 8

			ip := net.IP(data[addrOffset : addrOffset+net.IPv4len])

			portBytes := data[portOffset : portOffset+2]
			port := int(portBytes[0])<<8 + int(portBytes[1])

			dstAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip.String(), port))

			work := &Work{
				ID:      id.GetSnowflakeID().String(),
				SrcAddr: addr,
				Input:   make(chan []byte, 1024),
				Output:  make(chan []byte, 1024),
				SrcConn: l,
			}

			if shared.IPDB != nil {
				country, err := shared.IPDB.Country(ip)
				if err != nil {
					mlog.Error(err.Error())
					continue
				}

				for _, filter := range shared.Filters {
					if country.Country.IsoCode == filter {
						data = data[10:]
						mlog.Debug("request udp to " + dstAddr.String())

						udp, err := net.DialUDP("udp", nil, dstAddr)
						if err != nil {
							mlog.Error(err.Error())
							continue
						}

						work.DstConn = udp
						break
					}
				}
			}

			if work.DstConn == nil {
				mlog.Debug("request udp to " + dstAddr.String() + " by " + quicConn.String())

				r := models.Request{
					Network: shared.NetworkUDP,
					ID:      work.ID,
				}

				i := models.InitialPacket{
					Protocol: shared.SOCKS,
					Request:  r,
				}

				payload, err := json.Marshal(i)
				if err != nil {
					mlog.Error(err.Error())
					continue
				}

				stream, err := quicConn.NewStream(ctx)
				if err != nil {
					mlog.Error(err.Error())
					continue
				}

				_, err = stream.Write(payload)
				if err != nil {
					mlog.Error(err.Error())
					continue
				}
				stream.Flush()

				var buf [15]byte
				_, err = stream.Read(buf[:])
				if err != nil {
					mlog.Error(err.Error())
					continue
				}

				p := io2.Pipe{
					Stream: stream,
				}

				work.DstConn = &p
			}

			mlog.Debug(fmt.Sprintf("write to %s with %d bytes", dstAddr.String(), n))

			hm.Store(addr.Network()+addr.String(), work)

			go work.write()
			go work.read()
			work.Input <- data
		}
	}
}

func tcp(ctx context.Context, conn net.Conn, localAddr *net.UDPAddr, quicConn *quic.Conn) {
	authRequest, err := socks5.ReadAuthRequest(conn)
	if err != nil {
		return
	}
	// 检查是否支持用户名密码认证
	var supportAuth bool
	for _, m := range authRequest.Methods {
		if m == 0x02 { // 0x02 表示用户名密码认证
			supportAuth = true
			break
		}
	}

	if supportAuth {
		err = socks5.WriteAuthResponse(conn, socks5.AuthResponse{Method: socks5.AuthTypeUsernamePassword})
		if err != nil {
			return
		}
		request, err := socks5.ReadUsernamePasswordAuthRequest(conn)
		if err != nil {
			return
		}
		mlog.Debug(request.Username + " " + request.Password)
		// 回复验证通过
		err = socks5.WriteUsernamePasswordAuthResponse(conn, socks5.UsernamePasswordAuthResponse{Status: socks5.ReplyCodeSuccess})
		if err != nil {
			return
		}
	} else {
		err = socks5.WriteAuthResponse(conn, socks5.AuthResponse{Method: socks5.AuthTypeNotRequired})
		if err != nil {
			return
		}
	}

	// 解析目标地址
	request, err := socks5.ReadRequest(conn)
	if err != nil {
		return
	}

	if request.Command == 1 {
		ip, err := net.LookupIP(request.Destination.AddrString())
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
					directSocks(request, conn)
					return
				}
			}
		}

		outboundSocks(ctx, request, conn, quicConn)
	} else if request.Command == 3 {
		//if request.Destination.IsValid() {
		//	source, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", localAddr.IP.String(),
		//		request.Destination.Port))
		//	if err != nil {
		//		zlog.Error(err.Error())
		//		return
		//	}
		//	log.Println("source", source.Network(), source.String())
		//	UDPAddrs.Store(source.Network()+source.String(), dial)
		//}

		err = socks5.WriteResponse(conn, socks5.Response{ReplyCode: socks5.ReplyCodeSuccess, Bind: metadata.Socksaddr{
			Addr: netip.AddrFrom4(localAddr.AddrPort().Addr().As4()),
			Port: uint16(localAddr.Port),
		}})
		if err != nil {
			mlog.Error("Failed to write SOCKS5 UDP ASSOCIATE response:", zap.Error(err))
			return
		}
		conn.Close()
	}
}

func outboundSocks(ctx context.Context, req socks5.Request, conn net.Conn, quicConn *quic.Conn) {
	mlog.Debug("request tcp to " + req.Destination.String() + " by " + quicConn.String())

	r := models.Request{
		Network: shared.NetworkTCP,
		Address: fmt.Sprintf("%s:%d", req.Destination.AddrString(), req.Destination.Port),
	}

	i := models.InitialPacket{
		Protocol: shared.SOCKS,
		Request:  r,
	}

	payload, err := json.Marshal(i)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	stream, err := quicConn.NewStream(ctx)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	_, err = stream.Write(payload)
	if err != nil {
		mlog.Error(err.Error())
		return
	}
	stream.Flush()

	var buf [15]byte
	_, err = stream.Read(buf[:])
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	// 响应客户端请求成功
	err = socks5.WriteResponse(conn, socks5.Response{ReplyCode: socks5.ReplyCodeSuccess})
	if err != nil {
		mlog.Error("Failed to write SOCKS5 request response:", zap.Error(err))
		return
	}

	p := io2.Pipe{
		Stream: stream,
	}

	io2.Copy(&p, conn)
}

func directSocks(req socks5.Request, conn net.Conn) {
	targetConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", req.Destination.AddrString(), req.Destination.Port))
	if err != nil {
		mlog.Error(err.Error())
		return
	}
	defer targetConn.Close()

	mlog.Debug("request tcp to " + req.Destination.String() + " direct")

	// 响应客户端请求成功
	err = socks5.WriteResponse(conn, socks5.Response{ReplyCode: socks5.ReplyCodeSuccess})
	if err != nil {
		mlog.Error("Failed to write SOCKS5 request response:", zap.Error(err))
		return
	}

	// 转发
	go func() {
		_, err := io.Copy(targetConn, conn)
		if err != nil {
			mlog.Error(err.Error())
		}
	}()
	_, err = io.Copy(conn, targetConn)
	if err != nil {
		mlog.Error(err.Error())
	}
}

type Work struct {
	ID      string
	SrcAddr *net.UDPAddr
	Input   chan []byte
	Output  chan []byte
	SrcConn *net.UDPConn
	DstConn io.ReadWriteCloser
}

func (w *Work) write() {
	defer close(w.Output)
	defer w.DstConn.Close()

	for {
		select {
		case v, ok := <-w.Input:
			if !ok {
				return
			}

			_, err := w.DstConn.Write(v)
			if err != nil {
				mlog.Error(err.Error())
				return
			}
		}
	}
}

func (w *Work) read() {
	defer close(w.Input)

	buff := make([]byte, 1500)

	for {
		n, err := w.DstConn.Read(buff)
		if err != nil {
			mlog.Error(err.Error())
			return
		}
		var p models.Packet

		err = json.Unmarshal(buff[:n], &p)
		if err != nil {
			mlog.Error(err.Error())
			continue
		}

		mlog.Debug("back response with " + strconv.Itoa(len(p.Content)) + "bytes")
		buffer := bytes.NewBuffer([]byte{0, 0, 0, 1})

		ipBytes := p.Addr.IP.To4()
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, uint16(p.Addr.Port))

		// Write the IP and port to the buffer
		buffer.Write(ipBytes)
		buffer.Write(portBytes)
		buffer.Write(p.Content)

		_, err = w.SrcConn.WriteToUDP(buffer.Bytes(), w.SrcAddr)
		if err != nil {
			mlog.Error(err.Error())
			return
		}
	}
}

var (
	hm sync.Map
)
