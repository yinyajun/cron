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

// 单次执行
// 允许重复？
// 执行历史 executions
// 活跃任务 running

const MaxOutputLength = 1000

type Execution struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Node       string    `json:"node"`
	Output     string    `json:"output"`
	Success    bool      `json:"success"`
	invalid    bool
}

func newExecution(name, node string) *Execution {
	return &Execution{
		ID:        uuid.New(),
		Name:      name,
		StartedAt: time.Now(),
		Node:      node,
	}
}

func (e *Execution) finishWith(output string, err error) {
	e.FinishedAt = time.Now()
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
	name     string
	receiver chan string
	jobs     map[string]Job

	execution store.KV
	history   store.List
	running   store.Set

	maxHistoryNum int64
	keyExecution  string
	keyHistory    string
	keyRunning    string

	mu sync.RWMutex
}

func NewExecutor(cli *redis.Client) *Executor {
	e := &Executor{
		receiver: make(chan string),
		jobs:     make(map[string]Job),

		execution: store.NewRedisKV(cli),
		history:   store.NewRedisList(cli),
		running:   store.NewRedisSet(cli),

		maxHistoryNum: 10,
		keyExecution:  "_exe",
		keyHistory:    "_exe_hist",
		keyRunning:    "_exe_running",
	}

	return e
}

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

func (f *Executor) Add(job Job) {
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

	var executions = make([]Execution, len(ids))

	for i, id := range ids {
		executions[i] = f.fetchExecution(id)
		time.Sleep(5 * time.Millisecond)
	}
	return executions, nil
}

func (f *Executor) History(jobName string) ([]Execution, error) {
	key := f.keyHistory + "_" + jobName
	ids, err := f.history.LRange(key)
	if err != nil {
		return nil, err
	}
	var executions = make([]Execution, len(ids))

	for i, id := range ids {
		executions[i] = f.fetchExecution(id)
		time.Sleep(5 * time.Millisecond)
	}
	return executions, nil
}

func (f *Executor) consume() {
	for name := range f.receiver {
		go f.executeTask(context.Background(), name)
	}
}

func (f *Executor) executeTask(context context.Context, jobName string) {
	execution := newExecution(jobName, f.name)
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

func (f *Executor) fetchExecution(id string) Execution {
	_uuid, err := uuid.Parse(id)
	if err != nil {
		return Execution{ID: _uuid, invalid: true}
	}

	bytes, err := f.execution.Get(f.keyExecution + "_" + id)
	if err != nil {
		return Execution{ID: _uuid, invalid: true}
	}

	var execution Execution

	if err := json.Unmarshal(bytes, &execution); err != nil {
		return Execution{ID: _uuid, invalid: true}
	}
	return execution
}

func (f *Executor) updateExecution(execution *Execution) {
	ser, _ := json.Marshal(execution)
	f.execution.Set(f.keyExecution+"_"+execution.ID.String(), ser)
}

func (f *Executor) addToRunning(execution *Execution) {
	f.running.SAdd(f.keyRunning, execution.ID.String())
}

func (f *Executor) remFromRunning(execution *Execution) {
	f.running.SRem(f.keyRunning, execution.ID.String())
}

func (f *Executor) updateHistory(execution *Execution) {
	key := f.keyHistory + "_" + execution.Name
	size, _ := f.history.LLen(key)
	if size >= f.maxHistoryNum {
		f.history.RPop(key)
	}
	f.history.LPush(key, execution.ID.String())
}
