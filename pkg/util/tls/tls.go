package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"myproxy/internal/mlog"
	"myproxy/pkg/shared"
	"myproxy/pkg/util/id"
	"os"
	"time"
)

var (
	cipherSuites = []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
	}
)

func GetTLSConfig(prefix int, host string, insecure bool) *tls.Config {
	switch prefix {
	case shared.ServerTLS:
		return newServerTLSConfig("", "", "")
	case shared.ClientTLS:
		return newClientTLSConfig("", "", "", host, insecure)
	}
	return nil
}

func GetTLSConfigWithCustom(prefix int, host string, crt string, key string, insecure bool) *tls.Config {
	switch prefix {
	case shared.ServerTLS:
		return newServerTLSConfig(crt, key, "")
	case shared.ClientTLS:
		return newClientTLSConfig(crt, key, "", host, insecure)
	}
	return nil
}

func newCertificate() tls.Certificate {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	serial := id.GetSnowflakeID().Int64()
	template := x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject: pkix.Name{
			Organization: []string{"Internet Widgits Pty Ltd"},
			CommonName:   "localhost",
		},
		DNSNames:              []string{"localhost", "*.local"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	crtDER, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&privateKey.PublicKey,
		privateKey)
	if err != nil {
		panic(err)
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		panic(err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: crtDER})

	tlsCert, err := tls.X509KeyPair(crtPEM, keyPEM)
	if err != nil {
		panic(err)
	}

	return tlsCert
}

func newServerTLSConfig(certPath, keyPath, caPath string) *tls.Config {
	base := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		CipherSuites: cipherSuites,
		NextProtos:   []string{"h3"},
	}

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

func newClientTLSConfig(certPath, keyPath, caPath, serverName string, insecure bool) *tls.Config {
	base := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		CipherSuites: cipherSuites,
		NextProtos:   []string{"h3"},
	}

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
	} else {
		systemPool, err := x509.SystemCertPool()
		if err != nil || systemPool == nil {
			systemPool = x509.NewCertPool()
		}
		base.RootCAs = systemPool
	}

	base.InsecureSkipVerify = insecure

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
