package cron

import (
	"encoding/json"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yinyajun/cron/store"
)

type Entry struct {
	Name    string `json:"node"`
	Spec    string `json:"spec"`
	Deleted bool   `json:"deleted,omitempty"`

	schedule cron.Schedule
}

func (e Entry) String() string {
	ser, _ := json.Marshal(e)
	return string(ser)
}

type Cron struct {
	entries  *Entries
	timeline store.Timeline

	actionCh    chan Action
	executionCh chan<- string
	stop        chan struct{}
}

func NewCron(
	entries *Entries,
	timeline store.Timeline,
	result chan<- string) *Cron {

	c := &Cron{
		entries:  entries,
		timeline: timeline,

		actionCh:    make(chan Action),
		executionCh: result,
		stop:        make(chan struct{}),
	}

	if c.entries == nil || c.timeline == nil {
		Logger.Fatalln("cron init failed")
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

func (c *Cron) Pause(name string) error {
	if err := c.timeline.Hide(name); err != nil {
		return err
	}
	Logger.Info("pause:", name)
	return nil
}

func (c *Cron) Activate(name string) error {
	if err := c.timeline.Display(name); err != nil {
		return err
	}
	Logger.Info("activate:", name)
	return nil
}

func (c *Cron) Events() ([]store.Event, error) { return c.timeline.Events() }

func (c *Cron) close() { c.stop <- struct{}{} }

func (c *Cron) restore() {
	events, err := c.timeline.Events()
	if err != nil {
		Logger.Error("restore ", err)
	}

	names := make([]string, len(events))
	for i := 0; i < len(names); i++ {
		names[i] = events[i].Name
	}

	if err := c.entries.Restore(names); err != nil {
		Logger.Error("restore ", err)
	}
	Logger.Debugf("restore %d events from timeline", len(events))
}

func (c *Cron) run() {
	c.restore()

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
					Logger.Error("run failed: ", err.Error())
				}

			case action := <-c.actionCh:
				timer.Stop()

				switch action.Type {
				case addType:
					c.entries.Add(action.Entry)
					c.entries.Broadcast(action)
					Logger.Debug("add: ", action.Entry)
				case removeType:
					c.entries.Remove(action.Entry.Name)
					c.entries.Broadcast(action)
					Logger.Debug("remove: ", action.Entry.Name)
				}
			case <-c.stop:
				timer.Stop()
				close(c.executionCh)
				return
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
			Logger.Error("dispense failed: ", err.Error())
			continue
		}
		if tryOK {
			c.executionCh <- event.Name
			Logger.Info("dispense: ", entry)
		}
	}
	return nil
}

func parseSchedule(spec string) (cron.Schedule, error) {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom |
		cron.Month | cron.Dow | cron.Descriptor)
	return parser.Parse(spec)
}
