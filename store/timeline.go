package store

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

type Event struct {
	Name      string
	Time      time.Time
	Displayed bool
}

func (e Event) IsEmpty() bool { return e.Name == "" }

type Timeline interface {
	// Add adds a new event to timeline
	Add(e Event) error
	// Remove removes an event to timeline
	Remove(name string) error
	// Hide hides the event
	Hide(name string) error
	// Display displays the event
	Display(name string) error

	// TryModify tries to change the event time to t (CAS operation)
	TryModify(e Event, t time.Time) (bool, error)

	// Find the event
	Find(name string) (Event, error)
	// FindEarliest finds the earliest displayed event
	FindEarliest() (Event, error)
	// FetchHistory fetches displayed events happens <= t
	FetchHistory(t time.Time) ([]Event, error)
	// Events  including hidden events
	Events() ([]Event, error)

	Close()
}

type redisTimeline struct {
	key string
	cli *redis.Client
}

func NewRedisTimeline(cli *redis.Client) Timeline {
	return &redisTimeline{
		cli: cli,
		key: "timeline",
	}
}

func (r redisTimeline) withKey(key string) { r.key = key }

func (r redisTimeline) Remove(name string) error {
	return r.cli.ZRem(context.Background(), r.key, name).Err()
}

func (r redisTimeline) Add(event Event) error {
	return r.cli.ZAdd(context.Background(), r.key, &redis.Z{
		Score:  float64(r.time2ts(event.Time, event.Displayed)),
		Member: event.Name,
	}).Err()
}

func (r redisTimeline) Hide(name string) error {
	cmd := r.cli.ZScore(context.Background(), r.key, name)
	if cmd.Err() != nil {
		return cmd.Err()
	}

	event := Event{Name: name}
	event.Time, event.Displayed = r.ts2time(int64(cmd.Val()))

	if !event.Displayed {
		return nil
	}

	event.Displayed = false
	return r.Add(event)
}

func (r redisTimeline) Display(name string) error {
	cmd := r.cli.ZScore(context.Background(), r.key, name)
	if cmd.Err() != nil {
		return cmd.Err()
	}

	event := Event{Name: name}
	event.Time, event.Displayed = r.ts2time(int64(cmd.Val()))

	if event.Displayed {
		return nil
	}

	event.Displayed = true
	return r.Add(event)
}

// Input:
// KEYS[1] -> key
// --
// ARGV[1] -> event.Name
// ARGV[2] -> event.Time
// ARGV[3] -> t
//
// Output:
// Returns 1 if successfully modified
// Returns 0 if entry already modified
var modifyCmd = redis.NewScript(`
if redis.call("ZSCORE", KEYS[1], ARGV[1]) ~= ARGV[2] then 
	return 0
end
redis.call("ZADD" , KEYS[1], ARGV[3], ARGV[1])
return 1
`)

func (r redisTimeline) TryModify(event Event, t time.Time) (bool, error) {
	keys := []string{
		r.key,
	}
	argv := []interface{}{
		event.Name,
		r.time2ts(event.Time, event.Displayed),
		r.time2ts(t, event.Displayed),
	}
	res, err := modifyCmd.Run(context.Background(), r.cli, keys, argv...).Result()
	if err != nil {
		return false, err
	}

	return reflect.ValueOf(res).Int() == 1, nil
}

func (r redisTimeline) Find(name string) (Event, error) {
	cmd := r.cli.ZScore(context.Background(), r.key, name)
	if cmd.Err() == redis.Nil {
		return Event{}, nil
	}
	if cmd.Err() != nil {
		return Event{}, cmd.Err()
	}

	event := Event{Name: name}
	event.Time, event.Displayed = r.ts2time(int64(cmd.Val()))

	return event, nil
}

func (r redisTimeline) FindEarliest() (Event, error) {
	res, err := r.cli.ZRangeByScoreWithScores(context.Background(),
		r.key,
		&redis.ZRangeBy{
			Min:   "0",
			Max:   "inf",
			Count: 1,
		}).Result()
	if err != nil {
		return Event{}, err
	}

	if len(res) == 0 {
		return Event{}, nil
	}

	ts := int64(res[0].Score)
	name := res[0].Member.(string)

	// limited for displayed events
	return Event{Name: name, Time: time.Unix(ts, 0), Displayed: true}, nil
}

func (r redisTimeline) FetchHistory(t time.Time) ([]Event, error) {
	res, err := r.cli.ZRangeByScoreWithScores(context.Background(), r.key,
		&redis.ZRangeBy{
			Min: "0",
			Max: strconv.Itoa(int(t.Unix())),
		}).Result()
	if err != nil {
		return nil, err
	}

	var events = make([]Event, len(res))

	for i, z := range res {
		events[i] = Event{
			Name:      z.Member.(string),
			Time:      time.Unix(int64(z.Score), 0),
			Displayed: true,
		}
	}

	// limited for displayed events
	return events, nil
}

func (r redisTimeline) Events() ([]Event, error) {
	res, err := r.cli.ZRangeByScoreWithScores(context.Background(), r.key,
		&redis.ZRangeBy{
			Min: "-inf",
			Max: "inf",
		}).Result()
	if err != nil {
		return nil, err
	}

	var events = make([]Event, len(res))

	for i, z := range res {
		event := Event{Name: z.Member.(string)}
		event.Time, event.Displayed = r.ts2time(int64(z.Score))
		events[i] = event
	}

	return events, nil
}

func (r redisTimeline) Close() { r.cli.Close() }

func (r redisTimeline) time2ts(t time.Time, displayed bool) int64 {
	if !displayed {
		return -t.Unix()
	}
	return t.Unix()
}

func (r redisTimeline) ts2time(ts int64) (time.Time, bool) {
	if ts > 0 {
		return time.Unix(ts, 0), true
	}
	return time.Unix(-ts, 0), false
}
