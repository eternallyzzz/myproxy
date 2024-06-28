package protocol

import (
	"context"
	"golang.org/x/net/quic"
	"myproxy/pkg/models"
	"myproxy/pkg/shared"
	"myproxy/pkg/util/tls"
	"time"
)

var (
	Transfer *models.Transfer
)

const (
	maxStreams = 100
	maxIdle    = time.Minute * 30
	keepAlive  = time.Second * 20
)

func GetEndpoint(addr *models.NetAddr) (*quic.Endpoint, error) {
	if addr == nil {
		return nil, nil
	}
	l, err := quic.Listen(shared.NetworkQUIC, addr.String(), getSrvCfg())
	if err != nil {
		return nil, err
	}
	return l, nil
}

func GetEndPointDial(ctx context.Context, endpoint *quic.Endpoint, addr *models.NetAddr) (*quic.Conn, error) {
	dial, err := endpoint.Dial(ctx, shared.NetworkQUIC, addr.String(), getCliCfg(addr.Address))
	return dial, err
}

func getSrvCfg() *quic.Config {
	q := quic.Config{
		TLSConfig: tls.GetTLSConfig(shared.ServerTLS, ""),
	}
	convertToQUIC(&q)

	if Transfer.TLS != nil {
		q.TLSConfig = tls.GetTLSConfigWithCustom(shared.ServerTLS, "", Transfer.TLS.Crt, Transfer.TLS.Key)
	}

	return &q
}

func getCliCfg(addr string) *quic.Config {
	q := quic.Config{
		TLSConfig: tls.GetTLSConfig(shared.ClientTLS, addr),
	}
	convertToQUIC(&q)
	return &q
}

func convertToQUIC(q *quic.Config) {
	if Transfer != nil {
		q.MaxStreamReadBufferSize = int64(Transfer.MaxStreamReadBufferSize << 20)
		q.MaxStreamWriteBufferSize = int64(Transfer.MaxStreamWriteBufferSize << 20)
		q.MaxConnReadBufferSize = int64(Transfer.MaxConnReadBufferSize << 20)
		q.MaxBidiRemoteStreams = int64(Transfer.MaxBidiRemoteStreams)
		q.MaxUniRemoteStreams = int64(Transfer.MaxUniRemoteStreams)
		q.HandshakeTimeout = Transfer.HandshakeTimeout * time.Second
		q.MaxIdleTimeout = Transfer.MaxIdleTimeout * time.Second
		q.KeepAlivePeriod = Transfer.KeepAlivePeriod * time.Second
		q.RequireAddressValidation = Transfer.RequireAddressValidation
	}
	if q.MaxIdleTimeout < 0 {
		q.MaxIdleTimeout = -1
	}

	if Transfer.MaxBidiRemoteStreams == 0 {
		q.MaxBidiRemoteStreams = maxStreams
	}

	if Transfer.MaxIdleTimeout == 0 {
		q.MaxIdleTimeout = maxIdle
	}

	if Transfer.KeepAlivePeriod == 0 {
		q.KeepAlivePeriod = keepAlive
	}
}
