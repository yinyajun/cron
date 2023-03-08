package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
	"github.com/yinyajun/cron"
)

var (
	logger *logrus.Logger
	nodes  = flag.String("nodes", "", "nodes")
	port   = flag.Int("port", 0, "port")
)

func init() {
	formatter := new(logrus.TextFormatter)
	formatter.TimestampFormat = time.RFC3339
	formatter.FullTimestamp = true
	logger = &logrus.Logger{
		Out:       os.Stdout,
		Formatter: formatter,
		Level:     logrus.TraceLevel,
	}
}

func main() {
	flag.Parse()

	config := memberlist.DefaultLANConfig()
	config.Name = fmt.Sprintf("node_%d", port)
	config.BindPort = *port
	config.AdvertisePort = *port

	cli := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	store := cron.NewRedisKV(cli, "_cron")
	timeline := cron.NewRedisTimeline("_cron", cli)
	entries := cron.NewGossipEntries(store, config, strings.Split(*nodes, ","))

	// consume result chan
	result := make(chan string)
	go func() {
		for name := range result {
			go func(name string) {
				fmt.Println(" cron.jpgrun task", name)
			}(name)
		}
	}()

	c := cron.NewCron(timeline, entries, logger, result)

	c.Add("@every 5s", "t1")
	c.Activate("t1")
	c.Add("@every 3s", "t2")
	c.Activate("t2")

	select {}
}
