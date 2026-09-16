package engine

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"go.etcd.io/bbolt"
)

// completionStore records which pieces passed verification so transfers
// resume without rehashing. Unlike the stock implementations it can forget
// torrents, so removed transfers do not linger in resume data.
type completionStore interface {
	storage.PieceCompletion
	Forget(ih metainfo.Hash) error
	ForgetAll() error
}

var completionBucket = []byte("completion")

type boltCompletion struct {
	db *bbolt.DB
}

func openBoltCompletion(path string) (*boltCompletion, error) {
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	// Losing the most recent completions after a crash only costs a recheck.
	db.NoSync = true
	return &boltCompletion{db: db}, nil
}

func pieceIndexKey(index int) []byte {
	var k [4]byte
	binary.BigEndian.PutUint32(k[:], uint32(index))
	return k[:]
}

func (c *boltCompletion) Get(pk metainfo.PieceKey) (cn storage.Completion, err error) {
	err = c.db.View(func(tx *bbolt.Tx) error {
		root := tx.Bucket(completionBucket)
		if root == nil {
			return nil
		}
		b := root.Bucket(pk.InfoHash[:])
		if b == nil {
			return nil
		}
		v := b.Get(pieceIndexKey(pk.Index))
		if v == nil {
			return nil
		}
		cn.Ok = true
		cn.Complete = len(v) == 1 && v[0] == 'c'
		return nil
	})
	return cn, err
}

func (c *boltCompletion) Set(pk metainfo.PieceKey, complete bool) error {
	return c.db.Update(func(tx *bbolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists(completionBucket)
		if err != nil {
			return err
		}
		b, err := root.CreateBucketIfNotExists(pk.InfoHash[:])
		if err != nil {
			return err
		}
		v := []byte{'i'}
		if complete {
			v[0] = 'c'
		}
		return b.Put(pieceIndexKey(pk.Index), v)
	})
}

func (c *boltCompletion) Forget(ih metainfo.Hash) error {
	return c.db.Update(func(tx *bbolt.Tx) error {
		root := tx.Bucket(completionBucket)
		if root == nil || root.Bucket(ih[:]) == nil {
			return nil
		}
		return root.DeleteBucket(ih[:])
	})
}

func (c *boltCompletion) ForgetAll() error {
	return c.db.Update(func(tx *bbolt.Tx) error {
		if tx.Bucket(completionBucket) == nil {
			return nil
		}
		return tx.DeleteBucket(completionBucket)
	})
}

func (c *boltCompletion) Persistent() bool {
	return true
}

func (c *boltCompletion) Close() error {
	return c.db.Close()
}

// memoryCompletion keeps completion for the current session only, used when
// the user chose not to remember torrents between launches.
type memoryCompletion struct {
	mu sync.Mutex
	m  map[metainfo.PieceKey]bool
}

func newMemoryCompletion() *memoryCompletion {
	return &memoryCompletion{m: make(map[metainfo.PieceKey]bool)}
}

func (c *memoryCompletion) Get(pk metainfo.PieceKey) (storage.Completion, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	complete, ok := c.m[pk]
	return storage.Completion{Ok: ok, Complete: complete}, nil
}

func (c *memoryCompletion) Set(pk metainfo.PieceKey, complete bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[pk] = complete
	return nil
}

func (c *memoryCompletion) Forget(ih metainfo.Hash) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.m {
		if k.InfoHash == ih {
			delete(c.m, k)
		}
	}
	return nil
}

func (c *memoryCompletion) ForgetAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.m)
	return nil
}

// Persistent reports that completion survives dropping and re-adding a
// torrent within the session, which is how pausing works.
func (c *memoryCompletion) Persistent() bool {
	return true
}

func (c *memoryCompletion) Close() error {
	return nil
}
