package activity

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/boltdb/bolt"
	"github.com/pydio/services/common/proto/activity"
)

type BoltImpl struct {
	InboxMaxSize int32
	db           *bolt.DB
}

func NewBoltImpl(fileName string, inboxMaxSize int32) (*BoltImpl, error) {
	bi := &BoltImpl{
		InboxMaxSize: inboxMaxSize,
	}
	db, err := bolt.Open(fileName, 0644, nil)
	if err != nil {
		return nil, err
	}
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("nodes"))
		if err != nil {
			return err
		}
		return nil
	})
	bi.db = db
	return bi, nil
}

func (b *BoltImpl) Close() {
	b.db.Close()
}

// Load a given sub-bucket
// Bucket are structured like this:
// users
//   -> USER_ID
//      -> inbox [all notifications triggered by subscriptions or explicit alerts]
//      -> outbox [all user activities history]
//      -> lastseen [id of the last inbox notification read]
//      -> subscriptions [list of other users following her activities, with status]
// nodes
//   -> NODE_ID
//      -> outbox [all node activities, including its children ones]
//      -> subscriptions [list of users following this node activity]
func (b *BoltImpl) getBucket(tx *bolt.Tx, createIfNotExist bool, ownerType OwnerType, ownerId string, bucketName BoxName) (*bolt.Bucket, error) {

	mainBucket := tx.Bucket([]byte(ownerType))
	if createIfNotExist {

		objectBucket, err := mainBucket.CreateBucketIfNotExists([]byte(ownerId))
		if err != nil {
			return nil, err
		}
		targetBucket, err := objectBucket.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return nil, err
		}
		return targetBucket, nil

	} else {

		objectBucket := mainBucket.Bucket([]byte(ownerId))
		if objectBucket == nil {
			return nil, nil
		}
		targetBucket := objectBucket.Bucket([]byte(bucketName))
		if targetBucket == nil {
			return nil, nil
		}
		return targetBucket, nil

	}

}

func (b *BoltImpl) PostActivity(ownerType OwnerType, ownerId string, boxName BoxName, object *activity.Object) error {

	err := b.db.Update(func(tx *bolt.Tx) error {

		bucket, err := b.getBucket(tx, true, ownerType, ownerId, boxName)
		if err != nil {
			return err
		}
		objectKey, _ := bucket.NextSequence()
		object.Id = fmt.Sprintf("/activity-%v", objectKey)

		jsonData, _ := json.Marshal(object)

		k := make([]byte, 8)
		binary.BigEndian.PutUint64(k, objectKey)
		return bucket.Put(k, jsonData)

	})
	return err

}

func (b *BoltImpl) UpdateSubscription(userId string, toObjectType OwnerType, toObjectId string, status bool) error {

	err := b.db.Update(func(tx *bolt.Tx) error {

		bucket, err := b.getBucket(tx, true, toObjectType, toObjectId, BoxSubscriptions)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(userId), []byte(strconv.FormatBool(status)))
	})
	return err

}

func (b *BoltImpl) ListSubscriptions(objectType OwnerType, objectIds []string) (userIds map[string]bool, err error) {

	userIds = make(map[string]bool)
	e := b.db.View(func(tx *bolt.Tx) error {

		for _, objectId := range objectIds {
			bucket, _ := b.getBucket(tx, false, objectType, objectId, BoxSubscriptions)
			if bucket == nil {
				continue
			}
			bucket.ForEach(func(k, v []byte) error {
				uId := string(k)
				status, _ := strconv.ParseBool(string(v))
				if _, exists := userIds[uId]; exists {
					return nil // Continue
				}
				userIds[uId] = status
				return nil
			})
		}

		return nil
	})

	return userIds, e
}

func (b *BoltImpl) ActivitiesFor(ownerType OwnerType, ownerId string, boxName BoxName, result chan *activity.Object, done chan bool) error {

	defer func() {
		done <- true
	}()
	if boxName == "" {
		boxName = BoxOutbox
	}
	var lastRead string

	b.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		bucket, _ := b.getBucket(tx, false, ownerType, ownerId, boxName)
		if bucket == nil {
			// Does not exists, just return
			return nil
		}
		c := bucket.Cursor()
		i := 0
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			if lastRead == "" {
				lastRead = string(k)
			}
			acObject := &activity.Object{}
			err := json.Unmarshal(v, acObject)
			if err == nil {
				i++
				result <- acObject
			} else {
				return err
			}
			if i > 1000 {
				break
			}
		}
		return nil
	})

	if ownerType == OwnerTypeUser && boxName == BoxInbox && len(lastRead) > 0 {
		// Store last read in dedicated box
		go func() {
			b.db.Update(func(tx *bolt.Tx) error {
				bucket, err := b.getBucket(tx, true, OwnerTypeUser, ownerId, BoxLastRead)
				if err != nil {
					return err
				}
				return bucket.Put([]byte("last"), []byte(lastRead))
			})
		}()
	}

	return nil

}

func (b *BoltImpl) CountUnreadForUser(userId string) int {

	var unread int
	b.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		lastKey := ""
		lastReadBucket, _ := b.getBucket(tx, false, OwnerTypeUser, userId, BoxLastRead)
		if lastReadBucket != nil {
			// Does not exists, just return
			lastReadKey := lastReadBucket.Get([]byte("last"))
			if lastReadKey != nil {
				lastKey = string(lastReadKey)
			}
		}

		bucket, _ := b.getBucket(tx, false, OwnerTypeUser, userId, BoxInbox)
		c := bucket.Cursor()
		for k, _ := c.Last(); k != nil && string(k) != lastKey; k, _ = c.Prev() {
			unread++
		}
		return nil
	})
	return unread

}

// Should be wired to "USER_DELETE" and "NODE_DELETE" events
// to remove (or archive?) deprecated queues
func (b *BoltImpl) Delete(ownerType OwnerType, ownerId string) error {

	err := b.db.Update(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(ownerType))
		if b == nil {
			return nil
		}
		idBucket := b.Bucket([]byte(ownerId))
		if idBucket == nil {
			return nil
		}
		return b.DeleteBucket([]byte(ownerId))

	})

	return err
}
