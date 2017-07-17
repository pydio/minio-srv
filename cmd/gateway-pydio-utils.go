/*
 * Minio Cloud Storage, (C) 2017 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"github.com/minio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"io"
	"path/filepath"
	"strings"
	"time"
)

func (l *pydioObjects) findMinioClientFor(bucket string, prefix string) (*minio.Core, bool) {

	dsName, _ := l.prefixToDataSourceName(prefix)
	if dsName == "" {
		return nil, true
	}
	if client, ok := l.Clients[dsName]; ok {
		return client, true
	} else {
		return nil, false
	}

}

func (l *pydioObjects) translateBucketAndPrefix(bucket string, prefix string) (clientBucket string, clientPrefix string) {

	dsName, newPrefix := l.prefixToDataSourceName(prefix)
	if dsName == "" {
		return "pydio", ""
	}
	var ok bool
	if clientBucket, ok = l.dsBuckets[dsName]; !ok {
		return "", ""
	}
	return clientBucket, newPrefix

}

func (l *pydioObjects) prefixToDataSourceName(prefix string) (dataSourceName string, newPrefix string) {
	if len(strings.Trim(prefix, "/")) == 0 {
		return "", ""
	}
	parts := strings.Split(strings.Trim(prefix, "/"), "/")
	dataSourceName = parts[0]
	if len(parts) > 1 {
		newPrefix = strings.Join(parts[1:], "/")
	} else {
		newPrefix = ""
	}
	return dataSourceName, newPrefix
}

// fromMinioClientObjectInfo converts minio ObjectInfo to gateway ObjectInfo
func fromPydioNodeObjectInfo(bucket string, dsName string, node *tree.Node) ObjectInfo {
	//userDefined := fromMinioClientMetadata(oi.Metadata)
	//userDefined["Content-Type"] = oi.ContentType
	cType := "application/octet-stream"
	userDefined := map[string]string{
		"Content-Type": cType,
	}

	nodePath := node.Path
	if node.Type == tree.NodeType_COLLECTION {
		nodePath += "/"
	}
	return ObjectInfo{
		Bucket:          bucket,
		Name:            nodePath,
		ModTime:         time.Unix(0, node.MTime*int64(time.Second)),
		Size:            node.Size,
		ETag:            canonicalizeETag(node.Etag),
		UserDefined:     userDefined,
		ContentType:     cType,
		ContentEncoding: "",
	}
}

func (l *pydioObjects) ListPydioObjects(bucket string, prefix string, delimiter string, maxKeys int) (objects []ObjectInfo, prefixes []string, err error) {

	clientBucket, _ := l.translateBucketAndPrefix(bucket, prefix)
	if clientBucket == "pydio" {
		// Level 0 : List datasources as folders
		for dsName, _ := range l.Clients {
			if dsName == common.PYDIO_THUMBSTORE_NAMESPACE {
				continue
			}
			prefixes = append(prefixes, dsName+"/")
		}
		return objects, prefixes, nil
	} else if clientBucket == common.PYDIO_THUMBSTORE_NAMESPACE {
		return objects, prefixes, nil
	}

	treePath := strings.TrimLeft(prefix, "/")
	dataSourceName, _ := l.prefixToDataSourceName(prefix)
	recursive := false
	if delimiter == "" {
		recursive = true
	}
	var FilterType tree.NodeType
	if maxKeys == 1 {
		// We probably want to get only the very first object here (for folders stats)
		log.Println("Should get only LEAF nodes")
		FilterType = tree.NodeType_LEAF
	}
	lNodeClient, err := l.TreeClient.ListNodes(context.Background(), &tree.ListNodesRequest{
		Node: &tree.Node{
			Path: treePath,
		},
		Recursive:  recursive,
		Limit:      int64(maxKeys),
		FilterType: FilterType,
	})
	if err != nil {
		return nil, nil, s3ToObjectError(traceError(err), bucket)
	}
	for {
		clientResponse, err := lNodeClient.Recv()

		if clientResponse == nil {
			break
		}

		if err != nil {
			break
		}

		objectInfo := fromPydioNodeObjectInfo(bucket, dataSourceName, clientResponse.Node)
		if clientResponse.Node.IsLeaf() {
			objects = append(objects, objectInfo)
		} else {
			prefixes = append(prefixes, objectInfo.Name)
		}

	}
	if len(objects) > 0 && strings.Trim(prefix, "/") != "" {
		prefixes = append(prefixes, strings.TrimLeft(prefix, "/"))
	}

	return objects, prefixes, nil
}

func (l *pydioObjects) HeadFakeArchiveObject(bucket string, object string, dataSourceName string) (ObjectInfo, error) {

	if strings.HasSuffix(object, ".zip") || strings.HasSuffix(object, ".tar") || strings.HasSuffix(object, ".tar.gz") {

		noext := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(object, ".zip"), ".gz"), ".tar")

		n, er := l.TreeClient.ReadNode(context.Background(), &tree.ReadNodeRequest{Node: &tree.Node{Path: noext}})
		if er == nil && n != nil {
			n.Node.Type = tree.NodeType_LEAF
			n.Node.Path = object
			n.Node.Size = -1 // This will avoid a Content-Length discrepancy
			n.Node.SetMeta("name", filepath.Base(object))
			log.Println("This is a zip, sending folder info instead:", n.Node)
			return fromPydioNodeObjectInfo(bucket, dataSourceName, n.Node), nil
		}
	}

	return ObjectInfo{}, errors.New("Not Found")

}

func (l *pydioObjects) GenerateArchiveFromKey(writer io.Writer, bucket string, key string) (bool, error) {

	if strings.HasSuffix(key, ".zip") || strings.HasSuffix(key, ".tar") || strings.HasSuffix(key, ".tar.gz") {

		noext := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(key, ".zip"), ".gz"), ".tar")

		n, er := l.TreeClient.ReadNode(context.Background(), &tree.ReadNodeRequest{Node: &tree.Node{Path: noext}})
		if er == nil && n != nil {
			// This is a folder, trigger a zip download
			var err error
			if strings.HasSuffix(key, ".zip") {
				log.Println("This is a zip, create a zip on the fly")
				err = l.ZipFromPydioObjects(writer, bucket, noext, 0)
			} else if strings.HasSuffix(key, ".tar") {
				log.Println("This is a tar, create a tar on the fly")
				err = l.TarballFromPydioObjects(writer, false, bucket, noext, 0)
			} else if strings.HasSuffix(key, ".tar.gz") {
				log.Println("This is a tar.gz, create a tar.gz on the fly")
				err = l.TarballFromPydioObjects(writer, true, bucket, noext, 0)
			}
			return true, err
		}
	}

	return false, nil
}

func (l *pydioObjects) ZipFromPydioObjects(output io.Writer, bucket string, prefix string, maxKeys int) error {

	z := zip.NewWriter(output)
	defer z.Close()
	objects, _, err := l.ListPydioObjects(bucket, prefix, "", maxKeys)
	if err != nil {
		return err
	}
	for _, o := range objects {
		path := strings.TrimPrefix(o.Name, prefix+"/")
		log.Println("Adding path to Zip", path)
		header := &zip.FileHeader{
			Name:               path,
			Method:             zip.Deflate,
			UncompressedSize64: uint64(o.Size),
		}
		header.SetMode(0777)
		header.SetModTime(o.ModTime)
		w, e := z.CreateHeader(header)
		if e != nil {
			log.Println("Error while creating path", path, e)
			continue
		}
		e1 := l.GetObject(o.Bucket, o.Name, 0, -1, w)
		if e1 != nil {
			log.Println("Error while getting object", path, e1)
			continue
		}
	}

	return nil

}

func (l *pydioObjects) TarballFromPydioObjects(output io.Writer, gzipFile bool, bucket string, prefix string, maxKeys int) error {

	var tw *tar.Writer
	if gzipFile {
		// set up the gzip writer
		gw := gzip.NewWriter(output)
		defer gw.Close()

		tw = tar.NewWriter(gw)
		defer tw.Close()
	} else {
		tw = tar.NewWriter(output)
		defer tw.Close()
	}

	objects, _, err := l.ListPydioObjects(bucket, prefix, "", maxKeys)
	if err != nil {
		return err
	}
	for _, o := range objects {
		path := strings.TrimPrefix(o.Name, prefix+"/")
		log.Println("Adding path to tarball", path)
		header := &tar.Header{
			Name:    path,
			ModTime: o.ModTime,
			Size:    o.Size,
			Mode:    0777,
		}
		e := tw.WriteHeader(header)
		if e != nil {
			log.Println("Error while creating path", path, e)
			continue
		}
		e1 := l.GetObject(o.Bucket, o.Name, 0, -1, tw)
		if e1 != nil {
			log.Println("Error while getting object and writing to tarball", path, e1)
			continue
		}
	}
	return nil

}
