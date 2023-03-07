package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/yinyajun/cron"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

type MyFormatter struct{}

var (
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorBlue  = "\033[34m"
	colorCyan  = "\033[36m"
)

func (m *MyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	ts := entry.Time.Unix()
	color := colorBlue
	switch entry.Level {
	case logrus.DebugLevel:
		color = colorCyan
	case logrus.InfoLevel:
		color = colorGreen
	case logrus.ErrorLevel:
		color = colorRed
	}

	b.WriteString(color)
	b.WriteString(fmt.Sprintf("[%d] [%s] %s", ts, entry.Level, entry.Message))
	b.WriteString(colorReset)
	b.WriteString("\n")

	return b.Bytes(), nil
}

var bindPort = flag.Int("port", 0, "")

type task struct{ name string }

func (t task) Run() { fmt.Println("run", t.name) }

func main() {
	flag.Parse()

	formatter := new(logrus.TextFormatter)
	formatter.TimestampFormat = time.RFC3339
	formatter.FullTimestamp = true
	logger := &logrus.Logger{
		Out:       os.Stdout,
		Formatter: formatter,
		Level:     logrus.TraceLevel,
	}

	cli := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	store := cron.NewRedisKV(cli, "_cron")

	timeline := cron.NewRedisTimeline("_cron", cli)
	executor := cron.NewChanExecutor(map[string]cron.Job{
		"t1": task{"t1"},
		"t2": task{"t2"},
		"t3": task{"t3"},
	})

	cfg := memberlist.DefaultLANConfig()
	cfg.Name = fmt.Sprintf("test_%d", *bindPort)
	cfg.AdvertisePort = *bindPort
	cfg.BindPort = *bindPort

	entries := cron.NewGossipEntries(store, cfg,
		[]string{"127.0.0.1:8001", "127.0.0.1:8002"})

	c := cron.New(timeline, executor, entries, cron.WithLogger(logger))

	c.Add("@every 5s", "t1")
	c.Activate("t1")
	c.Add("@every 3s", "t2")
	c.Activate("t2")

	select {}
}
