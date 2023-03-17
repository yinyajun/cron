package cron

import (
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"io/ioutil"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var Logger *logrus.Logger

func init() {
	formatter := new(logrus.TextFormatter)
	formatter.TimestampFormat = time.RFC3339
	formatter.FullTimestamp = true
	Logger = &logrus.Logger{
		Out:       os.Stdout,
		Formatter: formatter,
		Level:     logrus.TraceLevel,
	}
}

type BaseConf struct {
	HTTPAddr     string        `json:"http_addr"`
	RedisOptions redis.Options `json:"redis"`
}

type GossipConf struct {
	Network  string `json:"network"`
	NodeName string `json:"node_name"`
	BindAddr string `json:"bind_addr"`
	BindPort int    `json:"bind_port"`
}

type CustomConf struct {
	// redis key name
	KeyTimeline string `json:"key_timeline"`
	KeyEntry    string `json:"key_entry"`
	KeyExecutor string `json:"key_executor"`
	// executor
	MaxHistoryNum   int64 `json:"max_history_num"`
	MaxOutputLength int   `json:"max_output_length"`
}

type Conf struct {
	Base   BaseConf   `json:"base"`
	Gossip GossipConf `json:"gossip"`
	Custom CustomConf `json:"custom"`
}

func ReadConfig(conf *Conf, file string) {
	defer conf.WithDefault()
	data, err := ioutil.ReadFile(file)
	if err != nil {
		Logger.Warn("ReadConfig failed: ", err.Error())
		return
	}
	if err = json.Unmarshal(data, conf); err != nil {
		Logger.Warn("ReadConfig failed: ", err.Error())
		return
	}
}

func (c *Conf) WithDefault() {
	// base
	if c.Base.HTTPAddr == "" {
		c.Base.HTTPAddr = ":8080"
	}

	// gossip
	if c.Gossip.Network == "" {
		c.Gossip.Network = "LAN"
	}
	if c.Gossip.BindAddr == "" {
		c.Gossip.BindAddr = "0.0.0.0"
	}
	if c.Gossip.BindPort == 0 {
		c.Gossip.BindPort = 7946
	}
	if c.Gossip.NodeName == "" {
		hostname, _ := os.Hostname()
		c.Gossip.NodeName = hostname
	}

	// custom
	if c.Custom.KeyEntry == "" {
		c.Custom.KeyEntry = "_entry"
	}
	if c.Custom.KeyTimeline == "" {
		c.Custom.KeyTimeline = "_timeline"
	}
	if c.Custom.KeyExecutor == "" {
		c.Custom.KeyExecutor = "_exe"
	}
	if c.Custom.MaxHistoryNum == 0 {
		c.Custom.MaxHistoryNum = 5
	}
}
