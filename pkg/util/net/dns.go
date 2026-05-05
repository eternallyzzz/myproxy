package net

import (
	"net"
	"sync"
	"time"
)

type dnsEntry struct {
	ips       []net.IP
	expiresAt time.Time
}

var (
	dnsCache sync.Map
	dnsTTL   = 5 * time.Minute
)

func LookupIP(host string) ([]net.IP, error) {
	if entry, ok := dnsCache.Load(host); ok {
		e := entry.(*dnsEntry)
		if time.Now().Before(e.expiresAt) {
			return e.ips, nil
		}
		dnsCache.Delete(host)
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}

	dnsCache.Store(host, &dnsEntry{
		ips:       ips,
		expiresAt: time.Now().Add(dnsTTL),
	})

	return ips, nil
}
