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
	"myproxy/internal"
	"myproxy/internal/mlog"
	"myproxy/internal/router"
	io2 "myproxy/pkg/io"
	"myproxy/pkg/models"
	"myproxy/pkg/protocol"
	"myproxy/pkg/shared"
	"myproxy/pkg/util/id"
	net2 "myproxy/pkg/util/net"
	"net"
	"net/netip"
	"strconv"
	"sync"
)

func Inbound(ctx context.Context, inb *models.Inbound) {
	udpAddr, err := net.ResolveUDPAddr("udp", inb.AddrPort())

	l, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		mlog.Error(err.Error())
		return
	}
	defer func(l *net.UDPConn) {
		err := l.Close()
		if err != nil {
			return
		}
	}(l)

	mlog.Info("listening UDP on " + udpAddr.String())

	go listenUDP(ctx, l, inb)

	tl, err := net.Listen("tcp", inb.AddrPort())
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

		go handSocks(ctx, client, udpAddr, inb)
	}
}

func listenUDP(ctx context.Context, l *net.UDPConn, inb *models.Inbound) {
	defer func(l *net.UDPConn) {
		err := l.Close()
		if err != nil {
			return
		}
	}(l)

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
				DstAddr: dstAddr,
				Input:   make(chan []byte, 1024),
				Output:  make(chan []byte, 1024),
				SrcConn: l,
			}

			r := router.Router{
				InboundTag: inb.Tag,
				DstAddr:    ip,
			}

			outTag := r.Process()

			if outTag == "direct" {
				data = data[10:]
				mlog.Debug("request udp to " + dstAddr.String())

				udp, err := net.DialUDP("udp", nil, dstAddr)
				if err != nil {
					mlog.Error(err.Error())
					continue
				}

				work.DstConn = udp
			}

			if work.DstConn == nil {
				info := internal.Osi[outTag]

				endpoint, err := protocol.GetEndpoint(&models.NetAddr{Port: net2.GetFreePort()})
				if err != nil {
					mlog.Error(err.Error())
					continue
				}

				dial, err := protocol.GetEndPointDial(ctx, endpoint, &models.NetAddr{Address: info.Address, Port: info.NodePort})
				if err != nil {
					mlog.Error(err.Error())
					continue
				}

				mlog.Debug("request udp to " + dstAddr.String() + " by " + dial.String())

				i := models.InitialPacket{
					Protocol: shared.SOCKS,
					Request: &models.Request{
						Network: shared.NetworkUDP,
						ID:      work.ID,
					},
				}

				payload, err := json.Marshal(i)
				if err != nil {
					mlog.Error(err.Error())
					continue
				}

				stream, err := dial.NewStream(ctx)
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

			go work.Write()
			go work.Read()
			work.Input <- data
		}
	}
}

func handSocks(ctx context.Context, conn net.Conn, localAddr *net.UDPAddr, inb *models.Inbound) {
	authRequest, err := socks5.ReadAuthRequest(conn)
	if err != nil {
		return
	}

	var supportAuth bool
	for _, m := range authRequest.Methods {
		if m == 0x02 {
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

		if inb.Setting != nil && inb.Setting.User != "" && inb.Setting.Pass != "" {
			if request.Username != inb.Setting.User || request.Password != inb.Setting.Pass {
				err := socks5.WriteUsernamePasswordAuthResponse(conn, socks5.UsernamePasswordAuthResponse{
					Status: socks5.UsernamePasswordStatusFailure,
				})
				if err != nil {
					return
				}
			}
		}

		err = socks5.WriteUsernamePasswordAuthResponse(conn, socks5.UsernamePasswordAuthResponse{
			Status: socks5.UsernamePasswordStatusSuccess,
		})
		if err != nil {
			return
		}
	} else {
		if inb.Setting != nil && inb.Setting.User != "" && inb.Setting.Pass != "" {
			err := socks5.WriteAuthResponse(conn, socks5.AuthResponse{
				Method: socks5.AuthTypeUsernamePassword,
			})
			if err != nil {
				return
			}
		}
		err = socks5.WriteAuthResponse(conn, socks5.AuthResponse{Method: socks5.AuthTypeNotRequired})
		if err != nil {
			return
		}
	}

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

		r := router.Router{
			InboundTag: inb.Tag,
			DstAddr:    ip[0],
		}

		outTag := r.Process()

		if outTag == "direct" {
			directTcp(request, conn)
			return
		}

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

		outTcp(ctx, request, conn, dial)
	} else if request.Command == 3 {
		err = socks5.WriteResponse(conn, socks5.Response{ReplyCode: socks5.ReplyCodeSuccess, Bind: metadata.Socksaddr{
			Addr: netip.AddrFrom4(localAddr.AddrPort().Addr().As4()),
			Port: uint16(localAddr.Port),
		}})
		if err != nil {
			mlog.Error("Failed to write SOCKS5 UDP ASSOCIATE response:", zap.Error(err))
			return
		}
		err := conn.Close()
		if err != nil {
			return
		}
	}
}

func outTcp(ctx context.Context, req socks5.Request, conn io.ReadWriteCloser, QUICConn *quic.Conn) {
	mlog.Debug("request tcp to " + req.Destination.String() + " by " + QUICConn.String())

	i := models.InitialPacket{
		Protocol: shared.SOCKS,
		Request: &models.Request{
			Network: shared.NetworkTCP,
			Dst:     req.Destination,
		},
	}

	payload, err := json.Marshal(i)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	stream, err := QUICConn.NewStream(ctx)
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

	p := io2.Pipe{
		Stream: stream,
	}

	io2.Copy(&p, conn)
}

func directTcp(req socks5.Request, conn io.ReadWriteCloser) {
	targetConn, err := net.Dial("tcp", req.Destination.String())
	if err != nil {
		mlog.Error(err.Error())
		return
	}
	defer func(targetConn net.Conn) {
		err := targetConn.Close()
		if err != nil {
			return
		}
	}(targetConn)

	mlog.Debug("request tcp to " + req.Destination.String() + " direct")

	err = socks5.WriteResponse(conn, socks5.Response{ReplyCode: socks5.ReplyCodeSuccess})
	if err != nil {
		mlog.Error("Failed to write SOCKS5 request response:", zap.Error(err))
		return
	}

	io2.Copy(targetConn, conn)
}

type Work struct {
	ID      string
	SrcAddr *net.UDPAddr
	DstAddr *net.UDPAddr
	Input   chan []byte
	Output  chan []byte
	SrcConn *net.UDPConn
	DstConn io.ReadWriteCloser
}

func (w *Work) Write() {
	defer close(w.Output)
	defer func(DstConn io.ReadWriteCloser) {
		err := DstConn.Close()
		if err != nil {
			return
		}
	}(w.DstConn)

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

func (w *Work) Read() {
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
