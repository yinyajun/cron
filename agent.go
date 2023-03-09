package cron

import (
	"errors"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
	"github.com/yinyajun/cron/store"
)

var ErrJobNotExist = errors.New("corresponding job not exist")

type Agent struct {
	cron     *Cron
	executor *Executor
}

func NewAgent(cli *redis.Client,
	existing []string,
	conf *memberlist.Config,
	logger *logrus.Logger) *Agent {
	kv := store.NewRedisKV(cli)

	timeline := store.NewRedisTimeline(cli)
	entries := NewGossipEntries(kv, conf, existing)
	executor := NewExecutor(cli)
	cron := NewCron(timeline, entries, logger, executor.Trigger())

	return &Agent{
		cron:     cron,
		executor: executor,
	}
}

func (a *Agent) Run() {
	go a.executor.consume()
	go a.cron.run()
}

func (a *Agent) AddJob(job Job) {
	a.executor.Add(job)
}

func (a *Agent) Add(spec, jobName string) error {
	if !a.executor.Contain(jobName) {
		return ErrJobNotExist
	}

	return a.cron.Add(spec, jobName)
}

func (a *Agent) Active(jobName string) error {
	if !a.executor.Contain(jobName) {
		return ErrJobNotExist
	}

	return a.cron.Activate(jobName)
}

func (a *Agent) Pause(jobName string) error {
	return a.cron.Pause(jobName)
}

func (a *Agent) Remove(jobName string) error {
	return a.cron.Remove(jobName)
}

func (a *Agent) ExecuteOnce(jobName string) error {
	return a.cron.Do(jobName)
}

func (a *Agent) Schedule() ([]store.Event, error) {
	return a.cron.Events()
}

func (a *Agent) Running() ([]Execution, error) {
	return a.executor.Running()
}

func (a *Agent) History(jobName string) ([]Execution, error) {
	return a.executor.History(jobName)
}

func (a *Agent) Jobs() []string {
	return a.executor.Jobs()
}
