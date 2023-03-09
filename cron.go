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

	add     chan Update
	remove  chan Update
	trigger chan string
	result  chan<- string

	logger *logrus.Logger
}

func NewCron(
	timeline store.Timeline,
	entries Entries,
	logger *logrus.Logger,
	result chan<- string,
) *Cron {
	c := &Cron{
		entries:  entries,
		timeline: timeline,
		add:      make(chan Update),
		remove:   make(chan Update),
		result:   result,
		logger:   logger,
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
		Name:   name,
		Time:   time.Now(),
		Hidden: true,
	}

	entry := &Entry{
		Name:     name,
		Spec:     spec,
		schedule: schedule,
	}

	update := Update{
		Action: addAction,
		Entry:  entry,
	}

	if err := c.entries.Backup(update); err != nil {
		return err
	}

	if err := c.timeline.Add(event); err != nil {
		return err
	}

	c.add <- update

	return nil
}

// Remove removes an entry
func (c *Cron) Remove(name string) error {
	if err := c.timeline.Remove(name); err != nil {
		return err
	}

	update := Update{
		Action: removeAction,
		Entry:  &Entry{Name: name},
	}

	if err := c.entries.Backup(update); err != nil {
		return err
	}

	return nil
}

func (c *Cron) Pause(name string) error    { return c.timeline.Hide(name) }
func (c *Cron) Activate(name string) error { return c.timeline.Display(name) }

func (c *Cron) Do(name string) error {
	event := store.Event{
		Name:   name,
		Time:   time.Now(),
		Hidden: true,
	}

	if err := c.timeline.Add(event); err != nil {
		return err
	}

	c.trigger <- name
	return nil
}

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
			timer = time.NewTimer(1 * time.Second)
		} else {
			timer = time.NewTimer(e.Time.Sub(now))
		}

		for {

			select {
			case now = <-timer.C:
				if err := c.handleExpired(now); err != nil {
					c.logger.Error("handle expired tasks failed: ", err.Error())
				}

			case u := <-c.add:
				timer.Stop()

				if u.Action != addAction {
					return
				}
				c.entries.Add(u.Entry)
				c.entries.Broadcast(u)
				c.logger.Debug("add: ", u.Entry)

			case u := <-c.remove:
				timer.Stop()

				if u.Action != removeAction {
					return
				}
				c.entries.Remove(u.Entry.Name)
				c.entries.Broadcast(u)
				c.logger.Debug("remove: ", u.Entry.Name)

			case name := <-c.trigger:
				timer.Stop()

				c.logger.Debug("trigger: ", name)
			}
			break
		}
	}
}

func (c *Cron) handleExpired(now time.Time) error {
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
			c.result <- event.Name
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
