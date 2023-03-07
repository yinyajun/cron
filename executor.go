package cron

import (
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

type Executor interface {
	Push(name string) // can not block
}

type chanExecutor struct {
	jobs   map[string]cron.Job
	events chan string
	logger *logrus.Logger
}

func NewChanExecutor(jobs map[string]cron.Job) *chanExecutor {
	e := &chanExecutor{
		jobs:   jobs,
		events: make(chan string),
	}

	go func() {
		for event := range e.events {
			job, ok := e.jobs[event]
			if !ok {
				e.logger.Errorf("job %s not exist", event)
				continue
			}

			go job.Run()
		}
	}()

	return e
}

func (e *chanExecutor) Push(name string) { e.events <- name }
