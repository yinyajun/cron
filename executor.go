package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/yinyajun/cron/store"
)

const DefaultTimeLayout = "2006-01-02 15:04:05"

type Execution struct {
	ID         uuid.UUID   `json:"id"`
	Name       string      `json:"name"`
	StartedAt  string      `json:"started_at"`
	FinishedAt string      `json:"finished_at"`
	Node       string      `json:"node"`
	Result     interface{} `json:"result"`
	Success    bool        `json:"success"`
}

func newExecution(name, node string) *Execution {
	return &Execution{
		ID:        uuid.New(),
		Name:      name,
		StartedAt: time.Now().Format(DefaultTimeLayout),
		Node:      node,
	}
}

func (e *Execution) finishWith(result interface{}, err error) {
	e.FinishedAt = time.Now().Format(DefaultTimeLayout)
	if err == nil {
		e.Result = result
		e.Success = true
		return
	}
	e.Result = fmt.Sprintf("Error: %s", err.Error())
}

type Job interface {
	Name() string
	Owner() string
	Run(context.Context) (result interface{}, err error)
}

type Executor struct {
	node     string
	receiver chan string
	jobs     map[string]Job

	kv      store.KV
	history store.List
	running store.Set

	maxHistoryNum  int64
	maxOutputLimit int
	keyPrefix      string

	mu sync.RWMutex
	wg sync.WaitGroup
}

func NewExecutor(cli *redis.Client, node string) *Executor {
	e := &Executor{
		node:     node,
		receiver: make(chan string),
		jobs:     make(map[string]Job),

		kv:      store.NewRedisKV(cli),
		history: store.NewRedisList(cli),
		running: store.NewRedisSet(cli),
	}

	return e
}

func (f *Executor) WithKeyPrefix(key string)  { f.keyPrefix = key }
func (f *Executor) WithMaxHistoryNum(n int64) { f.maxHistoryNum = n }

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
	ids, err := f.running.SRange(f.runningKey())
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

func (f *Executor) close() { f.wg.Wait() }

func (f *Executor) consume() {
	for name := range f.receiver {
		go f.executeTask(context.Background(), name)
	}
}

func (f *Executor) executeTask(context context.Context, jobName string) {
	f.wg.Add(1)
	execution := newExecution(jobName, f.node)
	Logger.Debugf("[%s] begin", execution.ID)
	f.updateExecution(execution)
	f.addToRunning(execution)
	f.updateHistory(execution)

	var (
		result interface{}
		err    error
	)

	f.mu.RLock()
	job, ok := f.jobs[jobName]
	f.mu.RUnlock()

	defer func() {
		execution.finishWith(result, err)
		f.updateExecution(execution)
		f.remFromRunning(execution)
		f.wg.Done()
		Logger.Debugf("[%s] finish", execution.ID)
	}()

	if !ok {
		err = fmt.Errorf("task %s not exist", job)
		return
	}

	result, err = job.Run(context)
}

func (f *Executor) fetchExecutions(ids []string) []Execution {
	var executions = make([]Execution, 0, len(ids))

	var keys = make([]string, len(ids))
	for i, id := range ids {
		keys[i] = f.executionKey(id)
	}

	res, err := f.kv.MGet(keys...)
	if err != nil {
		Logger.Errorf("fetchExecutions failed: %s", err.Error())
	}

	for i, r := range res {
		ser, ok := r.(string)
		if !ok {
			Logger.Warn("fetchExecutions err", keys[i])
			continue
		}
		var execution Execution

		if err := json.Unmarshal([]byte(ser), &execution); err != nil {
			Logger.Warn("fetchExecutions err", keys[i])
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
	f.running.SAdd(f.runningKey(), execution.ID.String())
}

func (f *Executor) remFromRunning(execution *Execution) {
	f.running.SRem(f.runningKey(), execution.ID.String())
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
	return f.keyPrefix + "_hist_" + name
}

func (f *Executor) runningKey() string {
	return f.keyPrefix + "_running"
}

func (f *Executor) executionKey(id string) string {
	return f.keyPrefix + "_" + id
}
