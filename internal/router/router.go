package router

import (
	"go.uber.org/zap"
	"myproxy/internal"
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
	if r.DstAddr == nil {
		return getDefaultOutTag()
	}

	country, err := shared.IPDB.Country(r.DstAddr)
	if err != nil {
		mlog.Error("", zap.Error(err))
		return getDefaultOutTag()
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

	if outTag == "" {
		outTag = getDefaultOutTag()
	}

	return outTag
}

func getDefaultOutTag() string {
	var outTag string

	for _, info := range internal.Osi {
		outTag = info.Tag
		break
	}

	if outTag == "" {
		outTag = "direct"
	}

	return outTag
}
