package router

import (
	"go.uber.org/zap"
	"myproxy/internal"
	"myproxy/internal/mlog"
	"myproxy/pkg/models"
	"myproxy/pkg/shared"
	"net"
	"strings"
	"sync"
)

var (
	rules   []*models.Rule
	rulesMu sync.RWMutex
)

func Run(v []*models.Rule) {
	rulesMu.Lock()
	rules = v
	rulesMu.Unlock()
}

type Router struct {
	InboundTag  string
	OutboundTag string
	DstAddr     net.IP
}

func (r *Router) Process() string {
	if r.DstAddr == nil {
		return getDefaultOutTag()
	}

	if shared.IPDB == nil {
		return getDefaultOutTag()
	}

	country, err := shared.IPDB.Country(r.DstAddr)
	if err != nil {
		mlog.Error("", zap.Error(err))
		return getDefaultOutTag()
	}

	ctCode := "!" + country.Country.IsoCode

	var outTag string
	rulesMu.RLock()
	for _, rule := range rules {
		if r.InboundTag == rule.InTag {
			for _, ip := range rule.IP {
				if ctCode == strings.ToUpper(ip) {
					rulesMu.RUnlock()
					return "direct"
				}
			}
			outTag = rule.OutTag
		}
	}
	rulesMu.RUnlock()

	if outTag == "" {
		outTag = getDefaultOutTag()
	}

	return outTag
}

func getDefaultOutTag() string {
	var outTag string

	internal.RangeOsi(func(key string, info internal.OutSeverInfo) bool {
		outTag = info.Tag
		return false
	})

	if outTag == "" {
		outTag = "direct"
	}

	return outTag
}
