package router

import (
	"go.uber.org/zap"
	"myproxy/internal/mlog"
	"myproxy/pkg/models"
	"myproxy/pkg/shared"
	"net"
	"strings"
)

var rules []*models.Rule

func Run(v []*models.Rule) {
	rules = v
}

type Router struct {
	InboundTag  string
	OutboundTag string
	DstAddr     net.IP
}

func (r *Router) Process() string {
	country, err := shared.IPDB.Country(r.DstAddr)
	if err != nil {
		mlog.Error("", zap.Error(err))
		return "direct"
	}

	ctCode := "!" + country.Country.IsoCode

	var outTag string
	for _, rule := range rules {
		if r.InboundTag == rule.InTag {
			for _, ip := range rule.IP {
				if ctCode == strings.ToUpper(ip) {
					return "direct"
				}
			}
			outTag = rule.OutTag
		}
	}

	return outTag
}
