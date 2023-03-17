package cron

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/memberlist"
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

	conf Conf
	stop chan os.Signal
}

func NewAgent(conf Conf) *Agent {
	cli := redis.NewClient(&conf.RedisOptions)

	// gossip conf
	gossipConf := memberlist.DefaultLANConfig()
	if conf.GossipType == "Local" {
		gossipConf = memberlist.DefaultLocalConfig()
	}
	if conf.GossipType == "WAN" {
		gossipConf = memberlist.DefaultWANConfig()
	}
	if conf.BindAddr != "" {
		gossipConf.BindAddr = conf.BindAddr
	}
	if conf.BindPort != 0 {
		gossipConf.BindPort = conf.BindPort
	}
	if conf.NodeName == "" {
		hostname, _ := os.Hostname()
		conf.NodeName = hostname
	}
	gossipConf.Name = conf.NodeName

	timeline := store.NewRedisTimeline(cli)
	entries := NewGossipEntries(cli, gossipConf)
	executor := NewExecutor(cli, conf.NodeName)

	// apply other conf
	if conf.KeyEntry != "" {
		entries.WithKeyPrefix(conf.KeyEntry)
	}
	if conf.NodeName != "" {
		executor.WithNode(conf.NodeName)
	}
	if conf.MaxHistoryNum != 0 {
		executor.WithMaxHistoryNum(conf.MaxHistoryNum)
	}
	if conf.MaxOutputLength != 0 {
		executor.WithMaxOutputLength(conf.MaxOutputLength)
	}

	cron := NewCron(entries, timeline, executor.Receiver())

	return &Agent{
		cron:     cron,
		executor: executor,

		conf: conf,
		stop: make(chan os.Signal),
	}
}

func (a *Agent) Join(existing []string) { a.cron.entries.Join(existing) }

func (a *Agent) Run() {
	go a.executor.consume()
	go a.cron.run()
	go a.serveHTTP()

	signal.Notify(a.stop, syscall.SIGINT)
	s := <-a.stop
	Logger.Info("receive a signal ", s)
	Logger.Info("begin to shutdown ", s)
	a.close()
}

func (a *Agent) serveHTTP() {
	Logger.Info("start admin http server ", a.conf.HTTPAddr)
	Logger.Fatalln(http.ListenAndServe(a.conf.HTTPAddr, Router(a)))
}

func (a *Agent) close() {
	a.cron.close()
	a.executor.close()
	a.cron.timeline.Close()
	a.cron.entries.Close()
	Logger.Info("agent shutdown gracefully")
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
			Next:      event.Time.Format(DefaultTimeLayout),
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

type Conf struct {
	// redis
	RedisOptions redis.Options `json:"redis"`

	// custom key
	KeyTimeline string `json:"key_timeline"`
	KeyEntry    string `json:"key_entry"`
	KeyExecutor string `json:"key_executor"`

	// executor
	MaxHistoryNum   int64 `json:"max_history_num"`
	MaxOutputLength int   `json:"max_output_length"`

	// api
	HTTPAddr string `json:"http_addr"`

	// gossip cluster
	GossipType string `json:"gossip_type"`
	NodeName   string `json:"node_name"`
	BindAddr   string `json:"bind_addr"`
	BindPort   int    `json:"bind_port"`
}

func ParseConfig(file string) Conf {
	f, err := os.Open(file)
	if err != nil {
		Logger.Fatalln(err)
	}
	defer f.Close()

	conf := Conf{}
	if err = json.NewDecoder(f).Decode(&conf); err != nil {
		Logger.Fatalln(err)
	}

	Logger.Info("load config ok")
	return conf
}
