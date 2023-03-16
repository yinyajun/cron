package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/yinyajun/cron/store"
)

const MaxOutputLength = 1000

type Execution struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	StartedAt  string    `json:"started_at"`
	FinishedAt string    `json:"finished_at"`
	Node       string    `json:"node"`
	Output     string    `json:"output"`
	Success    bool      `json:"success"`
}

func newExecution(name, node string) *Execution {
	return &Execution{
		ID:        uuid.New(),
		Name:      name,
		StartedAt: time.Now().Format("2006-01-02 15:04:05"),
		Node:      node,
	}
}

func (e *Execution) finishWith(output string, err error) {
	e.FinishedAt = time.Now().Format("2006-01-02 15:04:05")
	if err == nil {
		e.Output = output
		e.Success = true
		return
	}
	e.Output = fmt.Sprintf("Error: %s", err.Error())
}

type Job interface {
	Name() string
	Run(context.Context) (output string, err error)
}

type Executor struct {
	node     string
	receiver chan string
	jobs     map[string]Job

	kv      store.KV
	history store.List
	running store.Set

	maxHistoryNum int64
	keyExecution  string
	keyHistory    string
	keyRunning    string

	mu sync.RWMutex
}

func NewExecutor(cli *redis.Client) *Executor {
	hostname, _ := os.Hostname()
	e := &Executor{
		node:     hostname,
		receiver: make(chan string),
		jobs:     make(map[string]Job),

		kv:      store.NewRedisKV(cli),
		history: store.NewRedisList(cli),
		running: store.NewRedisSet(cli),

		maxHistoryNum: 4,
		keyExecution:  "_exe",
		keyHistory:    "_exe_hist",
		keyRunning:    "_exe_running",
	}

	return e
}

func (f *Executor) WithNode(node string) { f.node = node }

func (f *Executor) Receiver() chan string { return f.receiver }

func (f *Executor) Contain(jobName string) bool {
	_, ok := f.Get(jobName)
	return ok
}

func (f *Executor) Get(jobName string) (Job, bool) {
	f.mu.RLock()
	job, ok := f.jobs[jobName]
	f.mu.RUnlock()
	return job, ok
}

func (f *Executor) Register(job Job) {
	f.mu.Lock()
	f.jobs[job.Name()] = job
	f.mu.Unlock()
}

func (f *Executor) Jobs() []string {
	var jobs []string

	f.mu.RLock()
	for k := range f.jobs {
		jobs = append(jobs, k)
	}
	f.mu.RUnlock()

	return jobs
}

func (f *Executor) Running() ([]Execution, error) {
	ids, err := f.running.SRange(f.keyRunning)
	if err != nil {
		return nil, err
	}

	return f.fetchExecutions(ids), nil
}

func (f *Executor) History(jobName string) ([]Execution, error) {
	key := f.historyKey(jobName)
	ids, err := f.history.LRange(key)
	if err != nil {
		return nil, err
	}

	return f.fetchExecutions(ids), nil
}

func (f *Executor) consume() {
	for name := range f.receiver {
		go f.executeTask(context.Background(), name)
	}
}

func (f *Executor) executeTask(context context.Context, jobName string) {
	execution := newExecution(jobName, f.node)
	f.updateExecution(execution)
	f.addToRunning(execution)
	f.updateHistory(execution)

	var (
		output string
		err    error
	)

	f.mu.RLock()
	job, ok := f.jobs[jobName]
	f.mu.RUnlock()

	defer func() {
		execution.finishWith(output, err)
		f.updateExecution(execution)
		f.remFromRunning(execution)
	}()

	if !ok {
		err = fmt.Errorf("task %s not exist", job)
		return
	}

	output, err = job.Run(context)
	if len(output) > MaxOutputLength {
		output = output[:MaxOutputLength]
	}

}

func (f *Executor) fetchExecutions(ids []string) []Execution {
	var executions []Execution

	for _, id := range ids {
		ser, err := f.kv.Get(f.executionKey(id))
		if err != nil {
			// todo
			continue
		}

		var execution Execution

		if err := json.Unmarshal([]byte(ser), &execution); err != nil {
			// todo
			continue
		}
		executions = append(executions, execution)
	}
	return executions
}

func (f *Executor) updateExecution(execution *Execution) {
	ser, _ := json.Marshal(execution)
	f.kv.SetEx(f.executionKey(execution.ID.String()), ser, 48*time.Hour)
}

func (f *Executor) clearExecution(id string) {
	f.kv.Del(f.executionKey(id))
}

func (f *Executor) addToRunning(execution *Execution) {
	f.running.SAdd(f.keyRunning, execution.ID.String())
}

func (f *Executor) remFromRunning(execution *Execution) {
	f.running.SRem(f.keyRunning, execution.ID.String())
}

func (f *Executor) updateHistory(execution *Execution) {
	key := f.historyKey(execution.Name)
	size, _ := f.history.LLen(key)
	if size >= f.maxHistoryNum {
		if id, err := f.history.RPop(key); err == nil {
			f.clearExecution(id)
		}
	}
	f.history.LPush(key, execution.ID.String())
}

func (f *Executor) historyKey(name string) string {
	return f.keyHistory + "_" + name
}

func (f *Executor) executionKey(id string) string {
	return f.keyExecution + "_" + id
}
