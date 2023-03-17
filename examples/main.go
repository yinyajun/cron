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
	nodes = flag.String("nodes", "127.0.0.1", "existing cluster nodes")
	file  = flag.String("c", "./conf.json", "config file name")
	jobs  = []cron.Job{
		demoJob{"DemoJob1"},
		demoJob{"DemoJob2"},
		demoJob{"DemoJob3"},
		demoJob{"DemoJob4"},
		demoJob{"DemoJob5"},
		demoJob{"DemoJob6"},
		demoJob{"DemoJob7"},
		demoJob{"DemoJob8"},
		demoJob{"DemoJob9"},
		demoJob{"DemoJob10"},
		demoJob{"DemoJob11"},
		demoJob{"DemoJob12"},
	}
)

func init() {
	flag.Parse()
}

func main() {
	conf := &cron.Conf{}
	cron.ReadConfig(conf, *file)

	agent := cron.NewAgent(conf)
	agent.Join(strings.Split(*nodes, ","))

	if err := agent.Register(jobs...); err != nil {
		cron.Logger.Fatalln(err)
	}

	agent.Run()
}

type demoJob struct{ a string }

func (j demoJob) Name() string  { return j.a }
func (j demoJob) Owner() string { return "yinyajun" }
func (j demoJob) Run(ctx context.Context) (interface{}, error) {
	n := time.Duration(rand.Intn(5) + 3)
	time.Sleep(n * time.Second)
	if rand.Intn(10) >= 5 {
		return "run ok", nil
	}
	return "run failed", errors.New("unknown reason")
}
