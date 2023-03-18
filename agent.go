package cron

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/memberlist"
)

var (
	ErrJobNotSupport = errors.New("unsupported job")
	ErrJobNameEmpty  = errors.New("job node can not be empty")
)

type Agent struct {
	cron     *Cron
	executor *Executor
	server   http.Server

	stop chan os.Signal
}

func NewAgent(conf *Conf) *Agent {
	cli := redis.NewClient(&conf.Base.RedisOptions)

	// gossip
	gossipConf := memberlist.DefaultLANConfig()
	if conf.Gossip.Network == "Local" {
		gossipConf = memberlist.DefaultLocalConfig()
	}
	if conf.Gossip.Network == "WAN" {
		gossipConf = memberlist.DefaultWANConfig()
	}
	gossipConf.BindAddr = conf.Gossip.BindAddr
	gossipConf.BindPort = conf.Gossip.BindPort
	gossipConf.Name = conf.Gossip.NodeName

	timeline := NewRedisTimeline(cli, conf.Custom.KeyTimeline)
	entries := NewGossipEntries(cli, gossipConf)
	executor := NewExecutor(cli, entries.list.LocalNode().Name)
	cron := NewCron(entries, timeline, executor.Receiver())

	// custom
	entries.WithKeyPrefix(conf.Custom.KeyEntry)
	executor.WithKeyPrefix(conf.Custom.KeyExecutor)
	executor.WithMaxHistoryNum(conf.Custom.MaxHistoryNum)

	return &Agent{
		cron:     cron,
		executor: executor,
		server:   http.Server{Addr: conf.Base.HTTPAddr},

		stop: make(chan os.Signal),
	}
}

// Join must call before Run()
func (a *Agent) Join(existing []string) { a.cron.entries.Join(existing) }

func (a *Agent) Run() {
	go a.executor.consume()
	go a.cron.run()
	go a.serveHTTP()

	signal.Notify(a.stop, syscall.SIGINT)
	s := <-a.stop
	Logger.Infof("receive a signal %s, begin to shutdown...", s.String())
	a.close()
}

func (a *Agent) Register(jobs ...Job) error {
	for _, job := range jobs {
		if err := a.register(job); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) register(job Job) error {
	if job.Name() == "" {
		return ErrJobNameEmpty
	}
	a.executor.Register(job)
	return nil
}

func (a *Agent) serveHTTP() {
	a.server.Handler = a.Router()
	Logger.Info("start admin http server: ", a.server.Addr)
	Logger.Info(a.server.ListenAndServe())
}

func (a *Agent) close() {
	a.server.Close()
	a.cron.close()
	a.executor.close()
	a.cron.timeline.Close()
	a.cron.entries.Close()
	Logger.Info("agent shutdown gracefully")
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

	go a.executor.executeTask(context.Background(), jobName)
	Logger.Info("execute once:", jobName)
	return nil
}

func (a *Agent) Schedule() ([]entryRecord, error) {
	events, err := a.cron.Events()
	if err != nil {
		return nil, err
	}

	var results = make([]entryRecord, len(events))

	for i, event := range events {
		results[i] = entryRecord{
			Name:      event.Name,
			Next:      event.Time.Unix() * 1000,
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

func (a *Agent) History(jobName string, offset, size int64) ([]Execution, int64, error) {
	total := a.executor.maxHistoryNum
	if jobName == "" {
		return nil, total, ErrJobNameEmpty
	}
	executions, err := a.executor.History(jobName, offset, size)
	return executions, total, err
}

func (a *Agent) Jobs() []string {
	return a.executor.Jobs()
}

func (a *Agent) Members() []*memberlist.Node {
	return a.cron.entries.list.Members()
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

type entryRecord struct {
	Name      string `json:"name"`
	Spec      string `json:"spec"`
	Next      int64  `json:"next"`
	Displayed bool   `json:"displayed"`
}
