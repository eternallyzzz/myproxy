package router

import (
	"myproxy/pkg/models"
)

var rules []*models.Rule

func Run(v []*models.Rule) {
	rules = v
}

type Router struct {
	InboundTag  string
	OutboundTag string
	DstAddr     string
}

func (r *Router) Process() {

}
