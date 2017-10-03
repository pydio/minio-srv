package endpoints

import (
	"github.com/pydio/minio-go"
	"os"
	"time"
	"path/filepath"
)

type S3FileInfo struct{
	Object minio.ObjectInfo
	isDir  bool
}

// base name of the file
func (s *S3FileInfo) Name() string{
	if s.isDir {
		return filepath.Base(s.Object.Key)
	}else{
		return s.Object.Key
	}
}

// length in bytes for regular files; system-dependent for others
func (s *S3FileInfo) Size() int64 {
	return s.Object.Size
}
// file mode bits
func (s *S3FileInfo) Mode() os.FileMode {
	if s.isDir{
		return os.ModePerm & os.ModeDir
	}else {
		return os.ModePerm
	}

}
// modification time
func (s *S3FileInfo) ModTime() time.Time {
	return s.Object.LastModified
}
// abbreviation for Mode().IsDir()
func (s *S3FileInfo) IsDir() bool        {
	return s.isDir
}

// underlying data source (can return nil)
func (s *S3FileInfo) Sys() interface{}   {
	return s.Object
}

func NewS3FileInfo(object minio.ObjectInfo) (*S3FileInfo){
	return &S3FileInfo{
		Object: object,
		isDir: false,
	}
}

func NewS3FolderInfo(object minio.ObjectInfo) (*S3FileInfo){
	return &S3FileInfo{
		Object: object,
		isDir: true,
	}
}
