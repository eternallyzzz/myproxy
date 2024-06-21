package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/oschwald/geoip2-golang"
	"github.com/spf13/viper"
	"myproxy/internal/mlog"
	"myproxy/pkg/models"
	"myproxy/pkg/protocol"
	"myproxy/pkg/shared"
	"os"
	"time"
)

func Init(path string) (*models.Config, error) {
	if c, err := loadConfig(path); err != nil {
		return nil, err
	} else {
		if c.Transfer != nil {
			protocol.Transfer = c.Transfer
		}

		readFile, err := content.ReadFile("cn.mmdb")
		if err != nil {
			return nil, err
		}

		db, err := geoip2.FromBytes(readFile)
		if err != nil {
			return nil, err
		}

		shared.IPDB = db

		return c, mlog.Init(c.Log)
	}
}

func loadConfig(path string) (*models.Config, error) {
	if path == "" {
		wd, _ := os.Getwd()
		path = wd + shared.ConfigBase
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

	var cfg models.Config

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
