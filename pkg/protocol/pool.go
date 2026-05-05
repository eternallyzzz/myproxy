package protocol

import (
	"context"
	"fmt"
	"golang.org/x/net/quic"
	"myproxy/pkg/models"
	net2 "myproxy/pkg/util/net"
	"sync"
)

type poolEntry struct {
	conn     *quic.Conn
	endpoint *quic.Endpoint
}

type ConnPool struct {
	mu    sync.Mutex
	conns map[string]*poolEntry
}

var defaultPool = &ConnPool{
	conns: make(map[string]*poolEntry),
}

func GetConn(ctx context.Context, remoteAddr *models.NetAddr) (*quic.Conn, error) {
	key := remoteAddr.String()
	defaultPool.mu.Lock()
	entry, ok := defaultPool.conns[key]
	defaultPool.mu.Unlock()

	if ok {
		return entry.conn, nil
	}

	ep, err := GetEndpoint(&models.NetAddr{Port: net2.GetFreePort()})
	if err != nil {
		return nil, err
	}

	conn, err := GetEndPointDial(ctx, ep, remoteAddr)
	if err != nil {
		return nil, err
	}

	defaultPool.mu.Lock()
	if entry, ok := defaultPool.conns[key]; ok {
		defaultPool.mu.Unlock()
		conn.Close()
		return entry.conn, nil
	}
	defaultPool.conns[key] = &poolEntry{conn: conn, endpoint: ep}
	defaultPool.mu.Unlock()
	return conn, nil
}

func RemoveConn(netAddr *models.NetAddr) {
	key := netAddr.String()
	defaultPool.mu.Lock()
	if entry, ok := defaultPool.conns[key]; ok {
		delete(defaultPool.conns, key)
		entry.conn.Close()
		entry.endpoint.Close(context.Background())
	}
	defaultPool.mu.Unlock()
}

func StreamPool(ctx context.Context, remoteAddr *models.NetAddr) (*quic.Stream, error) {
	conn, err := GetConn(ctx, remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("pool dial %s: %w", remoteAddr.String(), err)
	}

	stream, err := conn.NewStream(ctx)
	if err != nil {
		RemoveConn(remoteAddr)
		conn, err = GetConn(ctx, remoteAddr)
		if err != nil {
			return nil, fmt.Errorf("pool redial %s: %w", remoteAddr.String(), err)
		}
		return conn.NewStream(ctx)
	}
	return stream, nil
}
