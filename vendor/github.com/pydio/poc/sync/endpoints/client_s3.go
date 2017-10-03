package endpoints

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/pydio/minio-go"
	"github.com/satori/go.uuid"
	"log"
	"net/url"
	"os"
	"strings"

	"crypto/md5"
	"github.com/pydio/poc/sync/common"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/text/unicode/norm"
	"io"
	"time"
	"golang.org/x/net/context"
)

var (
	UserAgentAppName = "pydio.sync.client.s3"
	UserAgentVersion = "1.0"
)

// TODO
// For Minio, add an initialization for detecting empty
// folders and creating .__pydio hidden files
type S3Client struct {
	Mc                          MockableMinio
	Bucket                      string
	RootPath                    string
	ServerRequiresNormalization bool
}

func NewS3Client(url string, key string, secret string, bucket string, rootPath string) (*S3Client, error) {
	mc, e := minio.New(url, key, secret, false)
	mc.SetAppInfo(UserAgentAppName, UserAgentVersion)
	if e != nil {
		return nil, e
	}
	return &S3Client{
		Mc:       mc,
		Bucket:   bucket,
		RootPath: strings.TrimRight(rootPath, "/"),
	}, e
}

func (c *S3Client) GetEndpointInfo() common.EndpointInfo {

	return common.EndpointInfo{
		RequiresFoldersRescan: false,
		RequiresNormalization: c.ServerRequiresNormalization,
	}

}

func (c *S3Client) normalize(path string) string {
	if c.ServerRequiresNormalization {
		return string(norm.NFC.Bytes([]byte(path)))
	}
	return path
}

func (c *S3Client) denormalize(path string) string {
	if c.ServerRequiresNormalization {
		return string(norm.NFD.Bytes([]byte(path)))
	}
	return path
}

func (c *S3Client) getLocalPath(eventPath string) string {
	return strings.TrimPrefix(c.normalize(eventPath), c.normalize(c.RootPath))
}

func (c *S3Client) getFullPath(path string) string {
	path = c.denormalize(path)
	if c.RootPath == "" {
		return strings.TrimLeft(path, "/")
	} else {
		return strings.TrimLeft(c.RootPath+"/"+strings.TrimLeft(path, "/"), "/")
	}
}

func (c *S3Client) Stat(path string) (i os.FileInfo, err error) {
	objectInfo, e := c.Mc.StatObject(c.Bucket, c.getFullPath(path))
	if e != nil {
		// Try folder
		folderInfo, e2 := c.Mc.StatObject(c.Bucket, c.getFullPath(path)+"/.__pydio")
		if e2 != nil {
			return nil, e
		}
		return NewS3FolderInfo(folderInfo), nil
	}
	return NewS3FileInfo(objectInfo), nil
}

func (c *S3Client) CreateNode(ctx context.Context, node *tree.Node, updateIfExists bool) (err error) {
	if node.IsLeaf() {
		return errors.New("This is a DataSyncTarget, use PutNode for leafs instead of CreateNode")
	}
	hiddenPath := fmt.Sprintf("%v/.__pydio", c.getFullPath(node.Path))
	_, err = c.Mc.PutObject(c.Bucket, hiddenPath, strings.NewReader(node.Uuid), "text/plain")
	return err
}

func (c *S3Client) UpdateNode(ctx context.Context, node *tree.Node) (err error) {
	return c.CreateNode(ctx, node, true)
}

func (c *S3Client) DeleteNode(ctx context.Context, path string) (err error) {
	path = c.getFullPath(path)

	doneChan := make(chan struct{})
	defer close(doneChan)
	for object := range c.Mc.ListObjectsV2(c.Bucket, path, true, doneChan) {
		err = c.Mc.RemoveObject(c.Bucket, object.Key)
		if err != nil {
			log.Print("Error while deleting object ", object.Key)
			return err
		}
	}
	return err
}

func (c *S3Client) MoveNode(ctx context.Context, oldPath string, newPath string) (err error) {

	doneChan := make(chan struct{})
	defer close(doneChan)
	oldPath = c.getFullPath(oldPath)
	newPath = c.getFullPath(newPath)
	for object := range c.Mc.ListObjectsV2(c.Bucket, oldPath, true, doneChan) {
		targetKey := newPath + strings.TrimPrefix(object.Key, oldPath)
		destinationInfo, _ := minio.NewDestinationInfo(c.Bucket, targetKey, nil, nil)
		sourceInfo := minio.NewSourceInfo(c.Bucket, object.Key, nil)
		copyResult := c.Mc.CopyObject(destinationInfo, sourceInfo)
		if copyResult == nil {
			c.Mc.RemoveObject(c.Bucket, object.Key)
		} else {
			return copyResult
		}
	}
	return nil

}

func (c *S3Client) GetWriterOn(path string) (out io.WriteCloser, err error) {

	path = c.getFullPath(path)
	reader, out := io.Pipe()
	go func() {
		c.Mc.PutObject(c.Bucket, path, reader, "image/png")
		reader.Close()
	}()
	return out, nil

}

func (c *S3Client) WriteObject(path string, reader io.Reader) (err error) {

	path = c.getFullPath(path)
	_, e := c.Mc.PutObject(c.Bucket, path, reader, "image/png")
	log.Println("write object")
	log.Println(reader)
	log.Println(e)
	return e

}

func (c *S3Client) GetReaderOn(path string) (out io.ReadCloser, err error) {

	return c.Mc.GetObject(c.Bucket, c.getFullPath(path))

}

func (c *S3Client) Walk(walknFc common.WalkNodesFunc, pathes ...string) (err error) {

	ctx := context.Background()
	wrappingFunc := func(path string, info *S3FileInfo, err error) error {
		path = c.getLocalPath(path)
		node, test := c.LoadNode(ctx, path, !info.IsDir())
		if test != nil || node == nil {
			log.Println("Loading a not found node, ignoring", path)
			return nil
		}

		node.MTime = info.ModTime().Unix()
		node.Size = info.Size()
		node.Mode = int32(info.Mode())
		if !info.IsDir() {
			node.Etag = strings.Trim(info.Object.ETag, "\"")
		} else {
			node.Uuid = strings.Trim(info.Object.ETag, "\"")
		}

		walknFc(path, node, nil)
		return nil
	}

	if len(pathes) > 0 {
		for _, nPath := range pathes {
			// Go be send in concurrency?
			e := c.actualLsRecursive(c.getFullPath(nPath), wrappingFunc)
			if e != nil {
				return e
			}
		}
		return nil
	} else {
		return c.actualLsRecursive(c.RootPath, wrappingFunc)
	}
}

func (c *S3Client) actualLsRecursive(recursivePath string, walknFc func(path string, info *S3FileInfo, err error) error) (err error) {
	doneChan := make(chan struct{})
	defer close(doneChan)
	createdDirs := make(map[string]bool)
	log.Println("List all objects for path '"+recursivePath+"' in bucket ", c.Bucket)
	for objectInfo := range c.Mc.ListObjectsV2(c.Bucket, recursivePath, true, doneChan) {
		if objectInfo.Err != nil {
			log.Print("Received ", objectInfo.Err)
			return objectInfo.Err
		}
		folderKey := common.DirWithInternalSeparator(objectInfo.Key)
		if strings.HasSuffix(objectInfo.Key, ".__pydio") {
			// Create Fake Folder
			// log.Print("Folder Key is " , folderKey)
			if folderKey == "" || folderKey == "." {
				continue
			}
			if _, exists := createdDirs[folderKey]; exists {
				continue;
			}
			folderObjectInfo := objectInfo
			folderObjectInfo.ETag, _, _ = c.readOrCreateFolderId(folderKey)
			s3FileInfo := NewS3FolderInfo(folderObjectInfo)
			walknFc(c.normalize(folderKey), s3FileInfo, nil)
			createdDirs[folderKey] = true
			//previousDir = folderKey
			//continue
		}
		if c.isIgnoredFile(objectInfo.Key) {
			continue
		}
		if folderKey != "" && folderKey != "." {
			c.createFolderIdsWhileWalking(createdDirs, walknFc, folderKey, objectInfo.LastModified)
		}
		if objectInfo.ETag == "" && objectInfo.Size > 0 {
			var etagErr error
			objectInfo, etagErr = c.s3forceComputeEtag(objectInfo)
			if etagErr != nil {
				log.Println("Error trying to compute etag", etagErr)
				continue
			}
			log.Println("Object Info Now Has ETAG ", objectInfo.ETag)
		}
		s3FileInfo := NewS3FileInfo(objectInfo)
		walknFc(c.normalize(objectInfo.Key), s3FileInfo, nil)
	}
	return nil
}

// Will try to create .__pydio to avoid missing empty folders
func (c *S3Client) createFolderIdsWhileWalking(createdDirs map[string]bool, walknFc func(path string, info *S3FileInfo, err error) error, currentDir string, lastModified time.Time){

	parts := strings.Split(currentDir, "/")
	for i := 0; i < len(parts); i++ {
		testDir := strings.Join(parts[0:i+1], "/")
		if _,exists := createdDirs[testDir]; exists {
			continue
		}
		uid, created, _ := c.readOrCreateFolderId(testDir)
		dirObjectInfo := minio.ObjectInfo{
			ETag:         uid,
			Key:          testDir,
			LastModified: lastModified,
			Size:         0,
		}
		s3FolderInfo := NewS3FolderInfo(dirObjectInfo)
		walknFc(c.normalize(testDir), s3FolderInfo, nil)
		if created.Key != "" {
			walknFc(c.normalize(created.Key), NewS3FileInfo(created), nil)
		}

		createdDirs[testDir] = true
	}

}

func (c *S3Client) s3forceComputeEtag(objectInfo minio.ObjectInfo) (minio.ObjectInfo, error) {

	if objectInfo.Size == 0 {
		return objectInfo, nil
	}
	//log.Println("No Etag, try copying object " + c.Bucket + "/" + objectInfo.Key)

	var destinationInfo minio.DestinationInfo
	var sourceInfo minio.SourceInfo

	destinationInfo, _ = minio.NewDestinationInfo(c.Bucket, objectInfo.Key+"--COMPUTE_HASH", nil, nil)
	sourceInfo = minio.NewSourceInfo(c.Bucket, objectInfo.Key, nil)
	copyErr := c.Mc.CopyObject(destinationInfo, sourceInfo)
	if copyErr != nil {
		log.Println(copyErr)
		return objectInfo, copyErr
	}

	destinationInfo, _ = minio.NewDestinationInfo(c.Bucket, objectInfo.Key, nil, nil)
	sourceInfo = minio.NewSourceInfo(c.Bucket, objectInfo.Key+"--COMPUTE_HASH", nil)
	copyErr = c.Mc.CopyObject(destinationInfo, sourceInfo)
	if copyErr != nil {
		log.Println(copyErr)
		return objectInfo, copyErr
	}

	removeErr := c.Mc.RemoveObject(c.Bucket, objectInfo.Key+"--COMPUTE_HASH")
	if removeErr != nil {
		log.Println(copyErr)
		return objectInfo, copyErr
	}
	newInfo, e := c.Mc.StatObject(c.Bucket, objectInfo.Key)
	if e != nil {
		return objectInfo, e
	}
	return newInfo, nil

}

func (c *S3Client) LoadNode(ctx context.Context, path string, leaf ...bool) (node *tree.Node, err error) {
	var hash, uid string = "", ""
	var isLeaf bool = false
	var stat os.FileInfo
	var eStat error
	if len(leaf) > 0 {
		isLeaf = leaf[0]
	} else {
		stat, eStat = c.Stat(path)
		if eStat != nil {
			return nil, eStat
		} else {
			isLeaf = !stat.IsDir()
		}
	}
	uid, hash, err = c.getNodeIdentifier(path, isLeaf)
	if err != nil {
		return nil, err
	}
	nodeType := tree.NodeType_LEAF
	if !isLeaf {
		nodeType = tree.NodeType_COLLECTION
	}
	node = &tree.Node{
		Path: path,
		Type: nodeType,
		Uuid: uid,
		Etag: hash,
	}
	if stat != nil {
		node.MTime = stat.ModTime().Unix()
		node.Size = stat.Size()
		node.Mode = int32(stat.Mode())
	} else {
		node.MTime = time.Now().Unix()
	}
	return node, nil
}

func (c *S3Client) getNodeIdentifier(path string, leaf bool) (uid string, eTag string, e error) {
	if leaf {
		return c.getFileHash(c.getFullPath(path))
	} else {
		uid, _, e = c.readOrCreateFolderId(c.getFullPath(path))
		return uid, "", e
	}
}

func (c *S3Client) readOrCreateFolderId(folderPath string) (uid string, created minio.ObjectInfo, e error) {

	hiddenPath := fmt.Sprintf("%v/.__pydio", folderPath)
	object, err := c.Mc.GetObject(c.Bucket, hiddenPath)
	if err == nil {
		buf := new(bytes.Buffer)
		buf.ReadFrom(object)
		uid = buf.String()
		if len(strings.TrimSpace(uid)) > 0 {
			return uid, minio.ObjectInfo{}, nil
		}
	}
	// Does not exists
	// Create dir uuid now
	uid = fmt.Sprintf("%s", uuid.NewV4())
	log.Println("Will create hidden folder with path " + hiddenPath)
	size, _ := c.Mc.PutObject(c.Bucket, hiddenPath, strings.NewReader(uid), "text/plain")
	h := md5.New()
	io.Copy(h, strings.NewReader(uid))
	Etag := fmt.Sprintf("%x", h.Sum(nil))
	created = minio.ObjectInfo{
		ETag:         Etag,
		Key:          hiddenPath,
		LastModified: time.Now(),
		Size:         size,
		ContentType:  "text/plain",
	}
	return uid, created, nil

}

func (c *S3Client) getFileHash(path string) (uid string, hash string, e error) {
	objectInfo, e := c.Mc.StatObject(c.Bucket, path)
	if e != nil {
		return "", "", e
	}
	uid = objectInfo.Metadata.Get("X-Amz-Meta-Pydio-Node-Uuid")
	etag := strings.Trim(objectInfo.ETag, "\"")
	if len(etag) == 0 {
		var etagE error
		objectInfo, etagE = c.s3forceComputeEtag(objectInfo)
		if etagE != nil {
			return uid, "", etagE
		}
		etag = strings.Trim(objectInfo.ETag, "\"")
	}
	return uid, etag, nil
}

func (c *S3Client) Watch(recursivePath string) (*common.WatchObject, error) {

	eventChan := make(chan common.EventInfo)
	errorChan := make(chan error)
	doneChan := make(chan bool)

	log.Print("Watching bucket " + c.Bucket)
	// Extract bucket and object.
	//if err := isValidBucketName(bucket); err != nil {
	//	return nil, err
	//}

	// Flag set to set the notification.
	var events []string
	events = append(events, string(minio.ObjectCreatedAll))
	events = append(events, string(minio.ObjectRemovedAll))

	doneCh := make(chan struct{})

	// wait for doneChan to close the other channels
	go func() {
		<-doneChan

		close(doneCh)
		close(eventChan)
		close(errorChan)
	}()

	// Start listening on all bucket events.
	eventsCh := c.Mc.ListenBucketNotification(c.Bucket, c.getFullPath(recursivePath), "", events, doneCh)

	wo := &common.WatchObject{
		EventInfoChan: eventChan,
		ErrorChan:     errorChan,
		DoneChan:      doneChan,
	}

	// wait for events to occur and sent them through the eventChan and errorChan
	go func() {
		defer wo.Close()
		for notificationInfo := range eventsCh {
			if notificationInfo.Err != nil {
				if nErr, ok := notificationInfo.Err.(minio.ErrorResponse); ok && nErr.Code == "APINotSupported" {
					errorChan <- errors.New("API Not Supported")
					return
				}
				errorChan <- notificationInfo.Err
			}
			for _, record := range notificationInfo.Records {
				//bucketName := record.S3.Bucket.Name
				key, e := url.QueryUnescape(record.S3.Object.Key)
				if e != nil {
					errorChan <- e
					continue
				}
				objectPath := key
				folder := false
				var additionalCreate string
				if strings.HasSuffix(key, ".__pydio") {
					additionalCreate = objectPath
					objectPath = common.DirWithInternalSeparator(key)
					folder = true
				}
				if c.isIgnoredFile(objectPath, record) {
					continue
				}
				objectPath = c.getLocalPath(objectPath)
				if strings.HasPrefix(record.EventName, "s3:ObjectCreated:") {
					log.Printf("S3 Created %s - %d - %v", objectPath, record.S3.Object.Size, record.S3.Object)
					eventChan <- common.EventInfo{
						Time:           record.EventTime,
						Size:           record.S3.Object.Size,
						Etag:           record.S3.Object.ETag,
						Path:           objectPath,
						Folder:         folder,
						PathSyncSource: c,
						Type:           common.EventCreate,
						Host:           record.Source.Host,
						Port:           record.Source.Port,
						UserAgent:      record.Source.UserAgent,
						Metadata: 		record.RequestParameters,
					}
					if additionalCreate != "" {
						// Send also the .__pydio event
						log.Printf("S3 Created %v", additionalCreate)
						eventChan <- common.EventInfo{
							Time:           record.EventTime,
							Size:           record.S3.Object.Size,
							Etag:           record.S3.Object.ETag,
							Path:           additionalCreate,
							Folder:         false,
							PathSyncSource: c,
							Type:           common.EventCreate,
							Host:           record.Source.Host,
							Port:           record.Source.Port,
							UserAgent:      record.Source.UserAgent,
							Metadata: 		record.RequestParameters,
						}
					}

				} else if strings.HasPrefix(record.EventName, "s3:ObjectRemoved:") {
					log.Printf("S3 Removed %v", objectPath)
					eventChan <- common.EventInfo{
						Time:           record.EventTime,
						Path:           objectPath,
						Folder:         folder,
						PathSyncSource: c,
						Type:           common.EventRemove,
						Host:           record.Source.Host,
						Port:           record.Source.Port,
						UserAgent:      record.Source.UserAgent,
						Metadata: 		record.RequestParameters,
					}
					if additionalCreate != "" {
						log.Printf("S3 Removed %v", additionalCreate)
						eventChan <- common.EventInfo{
							Time:           record.EventTime,
							Path:           additionalCreate,
							Folder:         false,
							PathSyncSource: c,
							Type:           common.EventRemove,
							Host:           record.Source.Host,
							Port:           record.Source.Port,
							UserAgent:      record.Source.UserAgent,
							Metadata: 		record.RequestParameters,
						}
					}
				} else if record.EventName == minio.ObjectAccessedGet {
					eventChan <- common.EventInfo{
						Time:           record.EventTime,
						Size:           record.S3.Object.Size,
						Etag:           record.S3.Object.ETag,
						Path:           objectPath,
						PathSyncSource: c,
						Type:           common.EventAccessedRead,
						Host:           record.Source.Host,
						Port:           record.Source.Port,
						UserAgent:      record.Source.UserAgent,
						Metadata: 		record.RequestParameters,
					}
				} else if record.EventName == minio.ObjectAccessedHead {
					eventChan <- common.EventInfo{
						Time:           record.EventTime,
						Size:           record.S3.Object.Size,
						Etag:           record.S3.Object.ETag,
						Path:           objectPath,
						PathSyncSource: c,
						Type:           common.EventAccessedStat,
						Host:           record.Source.Host,
						Port:           record.Source.Port,
						UserAgent:      record.Source.UserAgent,
						Metadata: 		record.RequestParameters,
					}
				}
			}
		}
	}()

	return wo, nil

}

func (c *S3Client) isIgnoredFile(path string, record ...minio.NotificationEvent) bool {
	if len(record) > 0 && strings.Contains(record[0].Source.UserAgent, UserAgentAppName) {
		return true
	}
	if common.IsIgnoredFile(path) {
		return true
	}
	return false
}
