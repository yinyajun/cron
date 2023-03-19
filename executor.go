package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type Execution struct {
	ID         uuid.UUID   `json:"id"`
	Name       string      `json:"name"`
	StartedAt  int64       `json:"started_at"`
	FinishedAt int64       `json:"finished_at"`
	Node       string      `json:"node"`
	Result     interface{} `json:"result"`
	Success    bool        `json:"success"`
}

func (e *Execution) finishWith(result interface{}, err error) {
	e.FinishedAt = time.Now().Unix() * 1000
	if err == nil {
		e.Result = result
		e.Success = true
		return
	}
	e.Result = fmt.Sprintf("Error: %s", err.Error())
}

type Job interface {
	Name() string
	Run(context.Context) (result interface{}, err error)
}

type Executor struct {
	cli *redis.Client
	mu  sync.RWMutex
	wg  sync.WaitGroup

	node     string
	receiver chan string
	jobs     map[string]Job

	maxHistoryNum int64
	keyPrefix     string
}

func NewExecutor(cli *redis.Client, node string) *Executor {
	e := &Executor{
		cli: cli,

		node:     node,
		receiver: make(chan string),
		jobs:     make(map[string]Job),
	}

	return e
}

func (f *Executor) newExecution(name string) *Execution {
	return &Execution{
		ID:        uuid.New(),
		Name:      name,
		StartedAt: time.Now().Unix() * 1000,
		Node:      f.node,
	}
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
	ids, err := f.cli.SMembers(context.Background(), f.runningKey()).Result()
	if err != nil {
		return nil, err
	}
	return f.fetchExecutions(ids), nil
}

func (f *Executor) History(jobName string, offset, size int64) ([]Execution, error) {
	ids, err := f.cli.LRange(context.Background(),
		f.historyKey(jobName), offset, offset+size-1).Result()
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
	execution := f.newExecution(jobName)
	f.beginExecution(execution)

	var (
		result interface{}
		err    error
	)

	defer func() {
		execution.finishWith(result, err)
		f.finishExecution(execution)
	}()

	f.mu.RLock()
	job, ok := f.jobs[jobName]
	f.mu.RUnlock()

	if !ok {
		err = fmt.Errorf("task %s not exist", job)
		return
	}
	result, err = job.Run(context)
}

func (f *Executor) fetchExecutions(ids []string) []Execution {
	var executions = make([]Execution, 0, len(ids))
	if len(ids) == 0 {
		return executions
	}

	var keys = make([]string, len(ids))
	for i, id := range ids {
		keys[i] = f.executionKey(id)
	}

	res, err := f.cli.MGet(context.Background(), keys...).Result()
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

// Input:
// KEYS[1] -> execution key
// KEYS[2] -> running key
// KEYS[3] -> history key
// --
// ARGV[1] -> serialization of execution
// ARGV[2] -> execution ID
// ARGV[3] -> max history num - 2
var beginCmd = redis.NewScript(`
redis.call("SETEX", KEYS[1], 86400, ARGV[1])
redis.call("SADD", KEYS[2], ARGV[2])
redis.call("LTRIM", KEYS[3], 0, ARGV[3] ) 
redis.call("LPUSH", KEYS[3], ARGV[2])
`)

func (f *Executor) beginExecution(e *Execution) {
	f.wg.Add(1)
	ser, _ := json.Marshal(e)
	id := e.ID.String()
	keys := []string{
		f.executionKey(id),
		f.runningKey(),
		f.historyKey(e.Name),
	}
	argv := []interface{}{
		ser,
		id,
		f.maxHistoryNum - 2,
	}
	beginCmd.Run(context.Background(), f.cli, keys, argv...)
	Logger.Debugf("[%s] begin", e.ID)
}

// Input:
// KEYS[1] -> execution key
// KEYS[2] -> running key
// --
// ARGV[1] -> serialization of execution
// ARGV[2] -> execution ID
var finishCmd = redis.NewScript(`
redis.call("SETEX", KEYS[1], 86400, ARGV[1])
redis.call("SREM", KEYS[2], ARGV[2])
`)

func (f *Executor) finishExecution(e *Execution) {
	ser, _ := json.Marshal(e)
	id := e.ID.String()
	keys := []string{
		f.executionKey(id),
		f.runningKey(),
	}
	argv := []interface{}{
		ser,
		id,
	}
	finishCmd.Run(context.Background(), f.cli, keys, argv...)
	f.wg.Done()
	Logger.Debugf("[%s] finish", e.ID)
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
