package cron

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

var _ memberlist.Delegate = (*gossipEntries)(nil)

type Entries interface {
	// Backup stores an entry's update operation to remote
	Backup(Update) error
	// Restore restores entries by name
	Restore([]string) error
	// Broadcast broadcasts an entry's update operation to peers
	Broadcast(Update)

	Add(*Entry)
	Remove(string)
	Get(string) (*Entry, bool)
	Entries() map[string]*Entry
}

type gossipEntries struct {
	mu sync.RWMutex

	store  KVStore
	local  map[string]*Entry
	logger *logrus.Logger
	list   *memberlist.Memberlist
	q      *memberlist.TransmitLimitedQueue
}

func NewGossipEntries(
	store KVStore,
	config *memberlist.Config,
	existing []string,
) Entries {

	entries := &gossipEntries{
		local:  make(map[string]*Entry),
		store:  store,
		logger: logrus.New(),
	}

	config.Delegate = entries

	list, err := memberlist.Create(config)
	if err != nil {
		log.Fatalln(err)
	}

	if _, err := list.Join(existing); err != nil {
		log.Fatalln(err)
	}

	entries.list = list
	entries.q = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int { return list.NumMembers() },
	}

	return entries
}

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

	var update Update
	if err := json.Unmarshal(b, &update); err != nil {
		return
	}

	switch update.Action {
	case addAction:
		s.Add(update.Entry)
		s.logger.Debug("add by gossip: ", update.Entry)

	case removeAction:
		s.Remove(update.Entry.Name)
		s.logger.Debug("remove by gossip: ", update.Entry.Name)
	}

}

func (s *gossipEntries) Add(entry *Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.schedule == nil {
		entry.schedule, _ = parseSchedule(entry.Spec)
	}

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

func (s *gossipEntries) Get(name string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.local[name]
	return e, ok
}

func (s *gossipEntries) Entries() map[string]*Entry {
	m := make(map[string]*Entry)

	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.local {
		m[k] = &Entry{
			Name:     v.Name,
			Spec:     v.Spec,
			Deleted:  v.Deleted,
			schedule: v.schedule,
		}
	}
	return m
}

func (s *gossipEntries) Broadcast(u Update) {
	b, _ := json.Marshal(u)

	s.q.QueueBroadcast(&broadcast{msg: b})
}

func (s *gossipEntries) Restore(names []string) error {
	for _, name := range names {
		e := &Entry{}
		ser, err := s.store.Read(name)
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

func (s *gossipEntries) Backup(u Update) error {
	switch u.Action {
	case addAction:
		ser, err := json.Marshal(u.Entry)
		if err != nil {
			return err
		}
		return s.store.Write(u.Entry.Name, ser)

	case removeAction:
		return s.store.Delete(u.Entry.Name)
	}
	return nil
}

type Action int

const (
	unkAction Action = iota
	addAction
	removeAction
)

type Update struct {
	Action Action `json:"action"`
	Entry  *Entry `json:"entry"`
}

type broadcast struct {
	msg []byte
}

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool { return false }
func (b *broadcast) Message() []byte                             { return b.msg }
func (b *broadcast) Finished()                                   {}

type KVStore interface {
	Write(name string, content []byte) error
	Read(name string) ([]byte, error)
	Delete(name string) error
}

type redisKV struct {
	cli       *redis.Client
	keyPrefix string
}

func NewRedisKV(cli *redis.Client, prefix string) KVStore {
	return redisKV{
		cli:       cli,
		keyPrefix: prefix,
	}
}

func (r redisKV) Write(key string, value []byte) error {
	return r.cli.Set(context.Background(), r._key(key), value, 0).Err()
}

func (r redisKV) Read(key string) ([]byte, error) {
	res, err := r.cli.Get(context.Background(), r._key(key)).Result()
	if err != nil {
		return nil, err
	}
	return []byte(res), nil
}

func (r redisKV) Delete(key string) error {
	return r.cli.Del(context.Background(), r._key(key)).Err()
}

func (r redisKV) _key(name string) string {
	return r.keyPrefix + "_" + name
}
