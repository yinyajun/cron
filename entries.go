package cron

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/memberlist"
)

var _ memberlist.Delegate = (*Entries)(nil)

type Entries struct {
	cli *redis.Client

	mu    sync.RWMutex
	local map[string]*Entry
	list  *memberlist.Memberlist
	q     *memberlist.TransmitLimitedQueue

	keyPrefix string
}

func NewGossipEntries(
	cli *redis.Client,
	config *memberlist.Config,
) *Entries {
	entries := &Entries{
		cli:   cli,
		local: make(map[string]*Entry),
	}

	config.Delegate = entries

	list, err := memberlist.Create(config)
	if err != nil {
		Logger.Fatalln(err)
	}

	entries.list = list
	entries.q = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int { return list.NumMembers() },
	}

	return entries
}

func (s *Entries) WithKeyPrefix(prefix string) { s.keyPrefix = prefix }

func (s *Entries) Join(existing []string) {
	if _, err := s.list.Join(existing); err != nil {
		Logger.Fatalln(err)
	}
}

func (s *Entries) GetBroadcasts(overhead, limit int) [][]byte {
	return s.q.GetBroadcasts(overhead, limit)
}

func (s *Entries) LocalState(join bool) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, _ := json.Marshal(s.local)
	return b
}

func (s *Entries) MergeRemoteState(buf []byte, join bool) {
	if len(buf) == 0 {
		return
	}

	var remotes map[string]*Entry
	if err := json.Unmarshal(buf, &remotes); err != nil {
		return
	}

	for name, r := range remotes {
		e, ok := s.Get(name)
		if !ok {
			s.Add(r)
			Logger.Debug("add by push/pull: ", r)
			continue
		}

		if !e.Deleted && r.Deleted {
			s.Remove(name)
			Logger.Debug("delete by push/pull: ", r)
			continue
		}
	}
}

func (s *Entries) NodeMeta(limit int) []byte {
	return []byte{}
}

func (s *Entries) NotifyMsg(b []byte) {
	if len(b) == 0 {
		return
	}

	var update Action
	if err := json.Unmarshal(b, &update); err != nil {
		return
	}

	switch update.Type {
	case addType:
		s.Add(update.Entry)
		Logger.Debug("add by gossip: ", update.Entry)

	case removeType:
		s.Remove(update.Entry.Name)
		Logger.Debug("remove by gossip: ", update.Entry.Name)
	}

}

func (s *Entries) Add(entry *Entry) {
	if entry.schedule == nil {
		entry.schedule, _ = parseSchedule(entry.Spec)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.local[entry.Name] = entry
}

func (s *Entries) Remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.local[name]
	if !ok {
		return
	}

	entry.Deleted = true
}

func (s *Entries) Get(name string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.local[name]
	return *e, ok
}

func (s *Entries) Entries() map[string]Entry {
	m := make(map[string]Entry)

	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.local {
		m[k] = Entry{
			Name:     v.Name,
			Spec:     v.Spec,
			Deleted:  v.Deleted,
			schedule: v.schedule,
		}
	}
	return m
}

func (s *Entries) Broadcast(u Action) {
	b, _ := json.Marshal(u)

	s.q.QueueBroadcast(&broadcast{msg: b})
}

func (s *Entries) Restore(names []string) error {
	for _, name := range names {
		e := &Entry{}

		ser, err := s.cli.Get(context.Background(), s.backupKey(name)).Result()
		if err != nil {
			return err
		}

		if err = json.Unmarshal([]byte(ser), e); err != nil {
			return err
		}

		s.Add(e)
		Logger.Info("restore: ", e)
	}
	return nil
}

func (s *Entries) Backup(u Action) error {
	switch u.Type {
	case addType:
		ser, err := json.Marshal(u.Entry)
		if err != nil {
			return err
		}
		return s.cli.Set(context.Background(), s.backupKey(u.Entry.Name), ser, 0).Err()

	case removeType:
		return s.cli.Del(context.Background(), s.backupKey(u.Entry.Name)).Err()
	}
	return nil
}

func (s *Entries) Close() { s.list.Shutdown() }

func (s *Entries) backupKey(key string) string {
	return s.keyPrefix + "_" + key
}

type Type int

const (
	invalid Type = iota
	addType
	removeType
)

type Action struct {
	Type  Type   `json:"type"`
	Entry *Entry `json:"entry"`
}

type broadcast struct{ msg []byte }

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool { return false }
func (b *broadcast) Message() []byte                             { return b.msg }
func (b *broadcast) Finished()                                   {}
