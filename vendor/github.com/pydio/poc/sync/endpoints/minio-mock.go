package endpoints

import (
	"github.com/pydio/minio-go"
	"io"
)

type MockableMinio interface{
	StatObject(bucket string, path string) (minio.ObjectInfo, error)
	RemoveObject(bucket string, path string) (error)
	PutObject(bucket string, path string, reader io.Reader, contentType string) (n int64, err error)
	GetObject(bucket string, path string) (object *minio.Object, err error)
	ListObjectsV2(bucketName, objectPrefix string, recursive bool, doneCh <-chan struct{}) <-chan minio.ObjectInfo
	CopyObject(dest minio.DestinationInfo, source minio.SourceInfo) error
	ListenBucketNotification(bucketName, prefix, suffix string, events []string, doneCh <-chan struct{}) <-chan minio.NotificationInfo
}