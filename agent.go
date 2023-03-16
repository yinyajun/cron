package cron

import (
	"context"
	"errors"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
	"github.com/yinyajun/cron/store"
)

var (
	ErrJobNotSupport = errors.New("unsupported job")
	ErrJobNameEmpty  = errors.New("job node can not be empty")
)

type EntryRecord struct {
	Name      string `json:"name"`
	Spec      string `json:"spec"`
	Next      string `json:"next"`
	Displayed bool   `json:"displayed"`
}

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
	cron := NewCron(entries, timeline, logger, executor.Receiver())

	return &Agent{
		cron:     cron,
		executor: executor,
	}
}

func (a *Agent) Start() {
	go a.executor.consume()
	go a.cron.run()
}

func (a *Agent) RegisterJob(job Job) error {
	if job.Name() == "" {
		return ErrJobNameEmpty
	}
	a.executor.Register(job)
	return nil
}

func (a *Agent) Add(spec, jobName string) error {
	if err := a.validate(jobName); err != nil {
		return err
	}

	return a.cron.Add(spec, jobName)
}

func (a *Agent) Active(jobName string) error {
	if err := a.validate(jobName); err != nil {
		return err
	}

	return a.cron.Activate(jobName)
}

func (a *Agent) Pause(jobName string) error {
	if err := a.validate(jobName); err != nil {
		return err
	}

	return a.cron.Pause(jobName)
}

func (a *Agent) Remove(jobName string) error {
	if err := a.validate(jobName); err != nil {
		return err
	}

	return a.cron.Remove(jobName)
}

func (a *Agent) ExecuteOnce(jobName string) error {
	if err := a.validate(jobName); err != nil {
		return err
	}

	a.executor.executeTask(context.Background(), jobName)
	return nil
}

func (a *Agent) Schedule() ([]EntryRecord, error) {
	events, err := a.cron.Events()
	if err != nil {
		return nil, err
	}

	var results = make([]EntryRecord, len(events))

	for i, event := range events {
		results[i] = EntryRecord{
			Name:      event.Name,
			Next:      event.Time.Format("2006-01-02 15:04:05"),
			Displayed: event.Displayed,
		}
		if e, ok := a.cron.entries.Get(event.Name); ok {
			results[i].Spec = e.Spec
		}
	}
	return results, nil
}

func (a *Agent) Running() ([]Execution, error) {
	return a.executor.Running()
}

func (a *Agent) History(jobName string) ([]Execution, error) {
	if jobName == "" {
		return nil, ErrJobNameEmpty
	}
	return a.executor.History(jobName)
}

func (a *Agent) Jobs() []string {
	return a.executor.Jobs()
}

func (a *Agent) validate(jobName string) error {
	if jobName == "" {
		return ErrJobNameEmpty
	}
	if !a.executor.Contain(jobName) {
		return ErrJobNotSupport
	}
	return nil
}
