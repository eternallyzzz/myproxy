package configLoader

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/oschwald/geoip2-golang"
	"github.com/spf13/viper"
	"os"
	"testDemo/pkg/config"
	"testDemo/pkg/model"
	"testDemo/pkg/zlog"
	"time"
)

func Init(path string) (*model.Config, error) {
	if c, err := loadConfig(path); err != nil {
		return nil, err
	} else {
		if c.Quic != nil {
			config.QUICCfg = c.Quic
		}

		if len(c.Filter) != 0 {
			config.Filters = c.Filter
			file, err := content.ReadFile("cn.mmdb")
			if err != nil {
				return nil, err
			}

			open, err := geoip2.FromBytes(file)
			if err != nil {
				return nil, err
			}

			config.IPDB = open
		}

		return c, zlog.Init(c.Log)
	}
}

func loadConfig(path string) (*model.Config, error) {
	if path == "" {
		wd, _ := os.Getwd()
		path = wd + config.CfgBase
		if file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0777); err != nil {
			return nil, err
		} else {
			_ = file.Close()
		}
	}

	fmt.Println(time.Now().Format(time.RFC3339), "	INFO", "	Use config: "+path)

	viper.SetConfigFile(path)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg model.Config

	m, err := json.Marshal(viper.AllSettings())
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(m, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

var (
	//go:embed cn.mmdb
	content embed.FS
)
