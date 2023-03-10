package main

import (
	"context"
	"errors"
	"flag"
	"log"
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

	nodes    = flag.String("nodes", "", "nodes")
	addr     = flag.String("addr", "", "redis_addr")
	password = flag.String("password", "", "redis_password")
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

	cli := redis.NewClient(&redis.Options{Addr: *addr, Password: *password})
	config := memberlist.DefaultLANConfig()
	nodes := strings.Split(*nodes, ",")

	agent := cron.NewAgent(
		cli,
		nodes,
		config,
		logger,
	)

	agent.RegisterJob(job{a: "t1"})
	agent.RegisterJob(job{a: "t2"})
	agent.RegisterJob(job{a: "t3"})
	agent.RegisterJob(job{a: "t4"})

	agent.Start()
	log.Fatalln(http.ListenAndServe(":8082", admin.NewHandler(agent)))
}

type job struct{ a string }

func (j job) Name() string {
	return j.a
}

func (j job) Run(ctx context.Context) (string, error) {
	time.Sleep(5 * time.Second)
	if rand.Intn(10) > 5 {
		return "run ok", nil
	}
	return "run failed", errors.New("error executed")
}
