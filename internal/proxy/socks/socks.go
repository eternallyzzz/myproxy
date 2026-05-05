package socks

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sagernet/sing/protocol/socks/socks5"
	"golang.org/x/net/quic"
	"myproxy/internal"
	"myproxy/internal/mlog"
	"myproxy/internal/router"
	"myproxy/pkg/io"
	"myproxy/pkg/models"
	"myproxy/pkg/protocol"
	"myproxy/pkg/shared"
	net2 "myproxy/pkg/util/net"
	"net"
	"sync"
)

func Process(ctx context.Context, r *models.Request, stream *quic.Stream) {
	switch r.Network {
	case shared.NetworkTCP:
		request := socks5.Request{
			Destination: r.Dst,
		}

		ips, err := net2.LookupIP(r.Dst.AddrString())
		if err != nil {
			mlog.Error(err.Error())
			return
		}
		if len(ips) == 0 {
			mlog.Error("no IPs resolved for " + r.Dst.AddrString())
			return
		}

		route := router.Router{DstAddr: ips[0]}
		outTag := route.Process()

		if outTag == "direct" {
			p := io.Pipe{
				Stream: stream,
			}

			directTcp(request, &p)
		} else {
			info, ok := internal.GetOsi(outTag)
			if !ok {
				mlog.Error("outbound not found: " + outTag)
				return
			}
			remoteAddr := &models.NetAddr{Address: info.Address, Port: info.NodePort}

			p := io.Pipe{Stream: stream}

			outTcp(ctx, request, &p, remoteAddr)
		}
		break
	case shared.NetworkUDP:
		route := router.Router{}
		outTag := route.Process()

		if outTag == "direct" {
			l, err := net.ListenUDP(r.Network, &net.UDPAddr{Port: int(net2.GetFreePort())})
			if err != nil {
				mlog.Error(err.Error())
				return
			}

			_, err = stream.Write([]byte("OK"))
			if err != nil {
				mlog.Error(err.Error())
				return
			}
			stream.Flush()

			handleStreamDirect(stream, l, r.ID)
		} else {
			handleStreamOut(ctx, stream, outTag, r.ID)
		}
		break
	}
}

func handleStreamDirect(stream *quic.Stream, l *net.UDPConn, id string) {
	buff := make([]byte, 1500)

	for {
		n, err := stream.Read(buff)
		if err != nil {
			mlog.Error(err.Error())
			return
		}

		data := buff[:n]

		if data[0] == 0 && data[1] == 0 {
			if len(data) < 10 {
				continue
			}
			addrOffset := 4
			portOffset := 8

			ip := net.IP(data[addrOffset : addrOffset+net.IPv4len])

			portBytes := data[portOffset : portOffset+2]
			port := int(portBytes[0])<<8 + int(portBytes[1])

			dstAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip.String(), port))
			data = data[10:]

			mlog.Debug("request udp to " + dstAddr.String())
			mlog.Debug(fmt.Sprintf("write to %s with %d bytes", dstAddr.String(), n))

			value, ok := dstHm.Load(id + dstAddr.String())
			if ok {
				work := value.(*DstWork)
				work.Input <- data
				continue
			}

			work := &DstWork{
				ID:      id,
				Input:   make(chan []byte, 1024),
				UDPConn: l,
				Stream:  stream,
				Dst:     dstAddr,
				Key:     id + dstAddr.String(),
			}

			go work.write()
			go work.read()
			dstHm.Store(id+dstAddr.String(), work)

			work.Input <- data
		}
	}
}

func handleStreamOut(ctx context.Context, src *quic.Stream, outTag, id string) {
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
	defer func(newStream *quic.Stream) {
		err := newStream.Close()
		if err != nil {
			return
		}
	}(newStream)

	i := models.InitialPacket{
		Protocol: shared.SOCKS,
		Request: &models.Request{
			Network: shared.NetworkUDP,
			ID:      id,
		},
	}

	payload, err := json.Marshal(i)
	if err != nil {
		mlog.Error(err.Error())
		return
	}

	_, err = newStream.Write(payload)
	if err != nil {
		mlog.Error(err.Error())
		return
	}
	newStream.Flush()

	input := io.Pipe{Stream: src}
	output := io.Pipe{Stream: newStream}

	io.Copy(&output, &input)
}

var dstHm sync.Map

type DstWork struct {
	ID      string
	Input   chan []byte
	UDPConn *net.UDPConn
	Stream  *quic.Stream
	Dst     *net.UDPAddr
	Key     string
}

func (d *DstWork) write() {
	defer dstHm.Delete(d.Key)
	defer func(UDPConn *net.UDPConn) {
		err := UDPConn.Close()
		if err != nil {
			return
		}
	}(d.UDPConn)
	defer func(Stream *quic.Stream) {
		err := Stream.Close()
		if err != nil {
			return
		}
	}(d.Stream)

	for {
		select {
		case v, ok := <-d.Input:
			if !ok {
				return
			}

			_, err := d.UDPConn.WriteToUDP(v, d.Dst)
			if err != nil {
				mlog.Error(err.Error())
				return
			}
		}
	}
}

func (d *DstWork) read() {
	defer close(d.Input)

	buff := make([]byte, 1500)

	for {
		n, addr, err := d.UDPConn.ReadFromUDP(buff)
		if err != nil {
			mlog.Error(err.Error())
			return
		}

		p := models.Packet{
			Content: buff[:n],
			Addr:    addr,
		}

		m, err := json.Marshal(&p)
		if err != nil {
			mlog.Error(err.Error())
			continue
		}
		_, err = d.Stream.Write(m)
		if err != nil {
			mlog.Error(err.Error())
			return
		}
		d.Stream.Flush()
	}
}
