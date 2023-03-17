package main

import (
	"context"
	"errors"
	"flag"
	"math/rand"
	"strings"
	"time"

	"github.com/yinyajun/cron"
)

var (
	nodes  = flag.String("nodes", "127.0.0.1", "nodes")
	config = flag.String("config", "./conf.json", "config")
)

func init() {
	flag.Parse()
}

func main() {
	conf := cron.ParseConfig(*config)
	agent := cron.NewAgent(conf)
	agent.Join(strings.Split(*nodes, ","))

	registerJob(agent)

	agent.Run()
}

func registerJob(agent *cron.Agent) {
	agent.RegisterJob(job{a: "t1"})
	agent.RegisterJob(job{a: "t2"})
	agent.RegisterJob(job{a: "t3"})
	agent.RegisterJob(job{a: "t4"})
	agent.RegisterJob(job{a: "t5"})
	agent.RegisterJob(job{a: "t6"})
	agent.RegisterJob(job{a: "t7"})
	agent.RegisterJob(job{a: "t8"})
	agent.RegisterJob(job{a: "t9"})
	agent.RegisterJob(job{a: "t10"})
	agent.RegisterJob(job{a: "t11"})
	agent.RegisterJob(job{a: "t12"})
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
