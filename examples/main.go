package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
	"github.com/yinyajun/cron"
	"github.com/yinyajun/cron/admin"
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

	agent := cron.NewAgent(
		cli,
		strings.Split(*nodes, ","),
		config,
		logger,
	)

	agent.AddJob(job{a: "t1"})
	agent.AddJob(job{a: "t2"})
	agent.AddJob(job{a: "t3"})

	agent.Start()

	h := admin.NewHandler(agent)
	http.ListenAndServe(":8081", h)
}

type job struct{ a string }

func (j job) Name() string {
	return j.a
}

func (j job) Run(ctx context.Context) (string, error) {
	fmt.Println("run", j.Name())
	time.Sleep(10 * time.Second)
	if rand.Intn(10) > 5 {
		return "", errors.New(j.a)
	}
	return j.a, nil
}
