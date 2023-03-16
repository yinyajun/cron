package cron

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
	"github.com/yinyajun/cron/store"
)

var _ memberlist.Delegate = (*gossipEntries)(nil)

type Entries interface {
	// Backup stores an entry's updateType operation to remote
	Backup(Action) error
	// Restore restores entries by node
	Restore([]string) error
	// Broadcast broadcasts an entry's updateType operation to peers
	Broadcast(Action)

	Add(*Entry)
	Remove(string)
	Get(string) (Entry, bool)
	Entries() map[string]Entry
}

type gossipEntries struct {
	mu sync.RWMutex

	prefix string
	kv     store.KV
	local  map[string]*Entry
	list   *memberlist.Memberlist
	q      *memberlist.TransmitLimitedQueue
	logger *logrus.Logger
}

func NewGossipEntries(
	kv store.KV,
	config *memberlist.Config,
	existing []string,
) Entries {

	entries := &gossipEntries{
		prefix: "_entries",
		local:  make(map[string]*Entry),
		kv:     kv,
		logger: logrus.New(),
	}

	config.Delegate = entries

	list, err := memberlist.Create(config)
	if err != nil {
		log.Fatalln(err)
	}

	// todo
	if _, err := list.Join(existing); err != nil {
		log.Fatalln(err)
	}

	entries.list = list
	entries.q = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int { return list.NumMembers() },
	}

	return entries
}

func (s *gossipEntries) WithPrefix(prefix string) { s.prefix = prefix }

func (s *gossipEntries) GetBroadcasts(overhead, limit int) [][]byte {
	return s.q.GetBroadcasts(overhead, limit)
}

func (s *gossipEntries) LocalState(join bool) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, _ := json.Marshal(s.local)
	return b
}

func (s *gossipEntries) MergeRemoteState(buf []byte, join bool) {
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
			s.logger.Debug("add by push/pull: ", r)
			continue
		}

		if !e.Deleted && r.Deleted {
			s.Remove(name)
			s.logger.Debug("delete by push/pull: ", r)
			continue
		}
	}
}

func (s *gossipEntries) NodeMeta(limit int) []byte {
	return []byte{}
}

func (s *gossipEntries) NotifyMsg(b []byte) {
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
		s.logger.Debug("add by gossip: ", update.Entry)

	case removeType:
		s.Remove(update.Entry.Name)
		s.logger.Debug("remove by gossip: ", update.Entry.Name)
	}

}

func (s *gossipEntries) Add(entry *Entry) {
	if entry.schedule == nil {
		entry.schedule, _ = parseSchedule(entry.Spec)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.local[entry.Name] = entry
}

func (s *gossipEntries) Remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.local[name]
	if !ok {
		return
	}

	entry.Deleted = true
}

func (s *gossipEntries) Get(name string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.local[name]
	return *e, ok
}

func (s *gossipEntries) Entries() map[string]Entry {
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

func (s *gossipEntries) Broadcast(u Action) {
	b, _ := json.Marshal(u)

	s.q.QueueBroadcast(&broadcast{msg: b})
}

func (s *gossipEntries) Restore(names []string) error {
	for _, name := range names {
		e := &Entry{}
		ser, err := s.kv.Get(s.backupKey(name))
		if err != nil {
			return err
		}

		if err = json.Unmarshal([]byte(ser), e); err != nil {
			return err
		}

		s.Add(e)
		s.logger.Debug("restore: ", e)
	}
	return nil
}

func (s *gossipEntries) Backup(u Action) error {
	switch u.Type {
	case addType:
		ser, err := json.Marshal(u.Entry)
		if err != nil {
			return err
		}
		return s.kv.SetEx(s.backupKey(u.Entry.Name), ser, 0)

	case removeType:
		return s.kv.Del(s.backupKey(u.Entry.Name))
	}
	return nil
}

func (s *gossipEntries) backupKey(key string) string {
	return s.prefix + "_" + key
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
