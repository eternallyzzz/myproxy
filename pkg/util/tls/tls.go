package tls

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"myproxy/internal/mlog"
	"myproxy/pkg/shared"
	"myproxy/pkg/util/id"
	"os"
)

func GetTLSConfig(prefix int, host string) *tls.Config {
	switch prefix {
	case shared.ServerTLS:
		return newServerTLSConfig("", "", "")
	case shared.ClientTLS:
		return newClientTLSConfig("", "", "", host)
	}
	return nil
}

func GetTLSConfigWithCustom(prefix int, host string, crt string, key string) *tls.Config {
	switch prefix {
	case shared.ServerTLS:
		return newServerTLSConfig(crt, key, "")
	case shared.ClientTLS:
		return newClientTLSConfig(crt, key, "", host)
	}
	return nil
}

func newCertificate() tls.Certificate {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	template := x509.Certificate{SerialNumber: big.NewInt(id.GetSnowflakeID().Int64())}
	crtDER, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		publicKey,
		privateKey)
	if err != nil {
		panic(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		panic(err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "ED25519 PRIVATE KEY", Bytes: key})
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: crtDER})

	tlsCert, err := tls.X509KeyPair(crtPEM, keyPEM)
	if err != nil {
		panic(err)
	}

	tlsCert.SupportedSignatureAlgorithms = []tls.SignatureScheme{tls.Ed25519}

	return tlsCert
}

func newServerTLSConfig(certPath, keyPath, caPath string) *tls.Config {
	base := &tls.Config{MinVersion: tls.VersionTLS13, CipherSuites: []uint16{tls.TLS_CHACHA20_POLY1305_SHA256}}

	if certPath == "" || keyPath == "" {
		cert := newCertificate()
		base.Certificates = []tls.Certificate{cert}
	} else {
		cert, err := newTLSKey(certPath, keyPath)
		if err != nil {
			mlog.Error(err.Error())
			return nil
		}

		base.Certificates = []tls.Certificate{*cert}
	}

	if caPath != "" {
		pool, err := newCertPool(caPath)
		if err != nil {
			mlog.Error(err.Error())
			return nil
		}

		base.ClientAuth = tls.RequireAndVerifyClientCert
		base.ClientCAs = pool
	}
	return base
}

func newClientTLSConfig(certPath, keyPath, caPath, serverName string) *tls.Config {
	base := &tls.Config{MinVersion: tls.VersionTLS13, CipherSuites: []uint16{tls.TLS_CHACHA20_POLY1305_SHA256}}

	if certPath != "" && keyPath != "" {
		cert, err := newTLSKey(certPath, keyPath)
		if err != nil {
			mlog.Error(err.Error())
			return nil
		}

		base.Certificates = []tls.Certificate{*cert}
	}

	base.ServerName = serverName

	if caPath != "" {
		pool, err := newCertPool(caPath)
		if err != nil {
			mlog.Error(err.Error())
			return nil
		}

		base.RootCAs = pool
		base.InsecureSkipVerify = false
	} else {
		base.InsecureSkipVerify = true
	}

	return base
}

func newTLSKey(certfile, keyfile string) (*tls.Certificate, error) {
	tlsCert, err := tls.LoadX509KeyPair(certfile, keyfile)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}

func newCertPool(caPath string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()

	caCrt, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	pool.AppendCertsFromPEM(caCrt)

	return pool, nil
}
