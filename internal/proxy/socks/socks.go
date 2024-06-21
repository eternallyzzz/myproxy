package socks

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/quic"
	"myproxy/internal/mlog"
	"myproxy/pkg/io"
	"myproxy/pkg/models"
	"myproxy/pkg/shared"
	net2 "myproxy/pkg/util/net"
	"net"
	"sync"
)

func Process(r models.Request, stream *quic.Stream) {
	switch r.Network {
	case shared.NetworkTCP:
		conn, err := net.Dial(r.Network, r.Address)
		if err != nil {
			mlog.Error(err.Error())
			return
		}
		mlog.Debug("request to " + r.Address)

		_, err = stream.Write([]byte("OK"))
		if err != nil {
			mlog.Error(err.Error())
			return
		}
		stream.Flush()

		p := io.Pipe{
			Stream: stream,
		}

		io.Copy(&p, conn)
		break
	case shared.NetworkUDP:
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

		handleStream(stream, l, r.ID)
		break
	}
}

func handleStream(stream *quic.Stream, l *net.UDPConn, id string) {
	buff := make([]byte, 1500)

	for {
		n, err := stream.Read(buff)
		if err != nil {
			mlog.Error(err.Error())
			return
		}

		data := buff[:n]

		if data[0] == 0 && data[1] == 0 {
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
				Input:   make(chan []byte, 1024),
				UDPConn: l,
				Stream:  stream,
				Dst:     dstAddr,
			}

			go work.write()
			go work.read()
			dstHm.Store(id+dstAddr.String(), work)

			work.Input <- data
		}
	}
}

var dstHm sync.Map

type DstWork struct {
	ID      string
	Input   chan []byte
	UDPConn *net.UDPConn
	Stream  *quic.Stream
	Dst     *net.UDPAddr
}

func (d *DstWork) write() {
	defer d.UDPConn.Close()
	defer d.Stream.Close()

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
