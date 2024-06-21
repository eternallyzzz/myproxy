package shared

import (
	"github.com/oschwald/geoip2-golang"
)

var (
	Filters []string
	IPDB    *geoip2.Reader
)
