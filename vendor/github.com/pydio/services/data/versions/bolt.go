package versions

import (
	"github.com/boltdb/bolt"
	"os"
	"github.com/pydio/services/common/proto/tree"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/micro/protobuf/proto"
	"context"
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"
)

var (
	bucketName = []byte("versions")
)

type BoltStore struct {
	// Internal DB
	db *bolt.DB
	// For Testing purpose : delete file after closing
	DeleteOnClose bool
	// Path to the DB file
	DbPath string
}

func NewBoltStore(fileName string, deleteOnClose ...bool) (*BoltStore, error) {

	bs := &BoltStore{
		DbPath: fileName,
	}
	if len(deleteOnClose) > 0 && deleteOnClose[0] {
		bs.DeleteOnClose = true
	}
	db, err := bolt.Open(fileName, 0644, nil)
	if err != nil {
		return nil, err
	}
	bs.db = db
	e2 := db.Update(func(tx *bolt.Tx) error {
		_, e := tx.CreateBucketIfNotExists(bucketName)
		return e
	})
	return bs, e2

}

func (b *BoltStore) Close() error {
	err := b.db.Close()
	if b.DeleteOnClose {
		os.Remove(b.DbPath)
	}
	return err
}

// Get Last version registered for this node
func (b *BoltStore) GetLastVersion(nodeUuid string) (log *tree.ChangeLog, err error) {

	err = b.db.View(func(tx *bolt.Tx) error {

		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return errors.NotFound(common.SERVICE_VERSIONS, "Bucket not found, this is not normal")
		}
		nodeBucket := bucket.Bucket([]byte(nodeUuid))
		if nodeBucket == nil {
			// Ignore not found
			return nil
		}
		c := nodeBucket.Cursor()
		_, v := c.Last()
		theLog := &tree.ChangeLog{}
		e := proto.Unmarshal(v, theLog)
		if e != nil {
			return e
		}
		log = theLog
		return nil
	})

	return log, err
}

// Get all versions from the node bucket, in reverse order (last inserted first)
func (b *BoltStore) GetVersions(nodeUuid string) (chan *tree.ChangeLog, chan bool) {

	logChan := make(chan *tree.ChangeLog)
	done := make(chan bool, 1)

	go func(){

		e := b.db.View(func(tx *bolt.Tx) error {

			defer func() {
				done <- true
				close(done)
			}()
			bucket := tx.Bucket(bucketName)
			if bucket == nil {
				return errors.NotFound(common.SERVICE_VERSIONS, "Bucket not found, this is not normal")
			}
			nodeBucket := bucket.Bucket([]byte(nodeUuid))
			if nodeBucket == nil {
				return errors.NotFound(common.SERVICE_VERSIONS, "No bucket found for this node")
			}
			c := nodeBucket.Cursor()

			for k, v := c.Last(); k != nil; k, v = c.Prev() {
				aLog := &tree.ChangeLog{}
				e := proto.Unmarshal(v, aLog)
				if e != nil {
					return e
				}
				log.Logger(context.Background()).Debug("Versions:Bolt", zap.Any("log", aLog))
				logChan <- aLog
			}

			return nil
		})
		if e != nil {
			log.Logger(context.Background()).Error("listVersions", zap.Error(e))
		}

	}()

	return logChan, done
}

// Store a version in the node bucket
func (b *BoltStore) StoreVersion(nodeUuid string, log *tree.ChangeLog) error {

	return b.db.Update(func(tx *bolt.Tx) error {

		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return errors.NotFound(common.SERVICE_VERSIONS, "Bucket not found, this is not normal")
		}
		nodeBucket, err := bucket.CreateBucketIfNotExists([]byte(nodeUuid))
		if err != nil {
			return err
		}
		newValue, e := proto.Marshal(log)
		if e != nil {
			return e
		}
		return nodeBucket.Put([]byte(log.Uuid), newValue)

	})

}

// Get a specific version in the node bucket
func (b *BoltStore) GetVersion(nodeUuid string, versionId string) (*tree.ChangeLog, error) {

	version := &tree.ChangeLog{}

	b.db.View(func(tx *bolt.Tx) error {

		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return errors.NotFound(common.SERVICE_VERSIONS, "Bucket not found, this is not normal")
		}
		nodeBucket := bucket.Bucket([]byte(nodeUuid))
		if nodeBucket == nil {
			return nil
		}
		data := nodeBucket.Get([]byte(versionId))
		return proto.Unmarshal(data, version)
	})

	return version, nil

}

// Delete whole node bucket at once
func (b *BoltStore) DeleteVersionsForNode(nodeUuid string) error {

	return b.db.Update(func(tx *bolt.Tx) error {

		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return errors.NotFound(common.SERVICE_VERSIONS, "Bucket not found, this is not normal")
		}
		nodeBucket := bucket.Bucket([]byte(nodeUuid))
		if nodeBucket != nil {
			return bucket.DeleteBucket([]byte(nodeUuid))
		}

		return nil
	})


}