package cron

import (
	"encoding/json"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/yinyajun/cron/store"
)

type Entry struct {
	Name    string `json:"name"`
	Spec    string `json:"spec"`
	Deleted bool   `json:"deleted,omitempty"`

	schedule cron.Schedule
}

func (e Entry) String() string {
	ser, _ := json.Marshal(e)
	return string(ser)
}

type Cron struct {
	entries  Entries
	timeline store.Timeline

	actionCh    chan Action
	executionCh chan<- string

	logger *logrus.Logger
}

func NewCron(
	entries Entries,
	timeline store.Timeline,
	logger *logrus.Logger,
	result chan<- string,
) *Cron {
	c := &Cron{
		entries:  entries,
		timeline: timeline,

		actionCh:    make(chan Action),
		executionCh: result,

		logger: logger,
	}

	if c.entries == nil || c.timeline == nil {
		c.logger.Fatalln("cron init failed")
	}

	if err := c.restore(); err != nil {
		c.logger.Error("restore failed: ", err)
	}

	return c
}

func (c *Cron) Add(spec string, name string) error {
	schedule, err := parseSchedule(spec)
	if err != nil {
		return err
	}

	event := store.Event{
		Name:      name,
		Time:      time.Now(),
		Displayed: false, // note: default state is paused
	}

	action := Action{
		Type:  addType,
		Entry: &Entry{Name: name, Spec: spec, schedule: schedule},
	}

	if err := c.entries.Backup(action); err != nil {
		return err
	}

	if err := c.timeline.Add(event); err != nil {
		return err
	}

	c.actionCh <- action

	return nil
}

func (c *Cron) Remove(name string) error {
	if err := c.timeline.Remove(name); err != nil {
		return err
	}

	update := Action{
		Type:  removeType,
		Entry: &Entry{Name: name},
	}

	if err := c.entries.Backup(update); err != nil {
		return err
	}

	return nil
}

func (c *Cron) Pause(name string) error    { return c.timeline.Hide(name) }
func (c *Cron) Activate(name string) error { return c.timeline.Display(name) }

func (c *Cron) Events() ([]store.Event, error) {
	return c.timeline.Events()
}

func (c *Cron) restore() error {
	events, err := c.timeline.Events()
	if err != nil {
		return err
	}

	names := make([]string, len(events))
	for i := 0; i < len(names); i++ {
		names[i] = events[i].Name
	}

	if err := c.entries.Restore(names); err != nil {
		return err
	}
	c.logger.Debugf("restore %d events from timeline", len(events))
	return nil
}

func (c *Cron) run() {
	var timer *time.Timer
	now := time.Now()

	for {
		if e, err := c.timeline.FindEarliest(); err != nil || e.IsEmpty() {
			timer = time.NewTimer(5 * time.Second)
		} else {
			timer = time.NewTimer(e.Time.Sub(now))
		}

		for {
			select {
			case now = <-timer.C:
				if err := c.doExpired(now); err != nil {
					c.logger.Error("run failed: ", err.Error())
				}

			case action := <-c.actionCh:
				timer.Stop()

				switch action.Type {
				case addType:
					c.entries.Add(action.Entry)
					c.entries.Broadcast(action)
					c.logger.Debug("add: ", action.Entry)
				case removeType:
					c.entries.Remove(action.Entry.Name)
					c.entries.Broadcast(action)
					c.logger.Debug("remove: ", action.Entry.Name)
				}

			}

			break
		}
	}
}

func (c *Cron) doExpired(now time.Time) error {
	expiredEvents, err := c.timeline.FetchHistory(now)
	if err != nil {
		return err
	}

	for _, event := range expiredEvents {
		entry, ok := c.entries.Get(event.Name)
		if !ok {
			continue
		}

		next := entry.schedule.Next(event.Time)
		// entry expires long ago
		if now.After(next) {
			next = entry.schedule.Next(now)
		}

		tryOK, err := c.timeline.TryModify(event, next)
		if err != nil {
			c.logger.Error("dispense failed: ", err.Error())
			continue
		}
		if tryOK {
			c.executionCh <- event.Name
			c.logger.Info("dispense: ", entry)
		}
	}
	return nil
}

func parseSchedule(spec string) (cron.Schedule, error) {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom |
		cron.Month | cron.Dow | cron.Descriptor)
	return parser.Parse(spec)
}
