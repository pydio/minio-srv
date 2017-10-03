package tree

import (
	"encoding/json"
	"io"
	"path/filepath"

	"go.uber.org/zap"

	"strings"
	"time"

	"path"

	"os"

	"github.com/micro/go-micro/errors"
	"github.com/micro/go-micro/metadata"
	"github.com/micro/protobuf/jsonpb"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"golang.org/x/net/context"
)

// IsLeaf checks if node is of type NodeType_LEAF or NodeType_COLLECTION
func (node *Node) IsLeaf() bool {
	return node.Type == NodeType_LEAF
}

// IsLeafInt checks if node is of type NodeType_LEAF or NodeType_COLLECTION, return as 0/1 integer (for storing)
func (node *Node) IsLeafInt() int {
	if node.Type == NodeType_LEAF {
		return 1
	}
	return 0
}

func (node *Node) GetModTime() time.Time {
	return time.Unix(0, node.MTime*int64(time.Second))
}

// GetMetaJSON gets a metadata and unmarshall it to JSON format
func (node *Node) GetMeta(namespace string, jsonStruc interface{}) error {
	metaString := node.getMetaString(namespace)
	if metaString == "" {
		return nil
	}
	return json.Unmarshal([]byte(metaString), &jsonStruc)
}

// SetMetaJSON sets a metadata by marshalling to JSON
func (node *Node) SetMeta(namespace string, jsonMeta interface{}) (err error) {
	if node.MetaStore == nil {
		node.MetaStore = make(map[string]string)
	}
	var bytes []byte
	bytes, err = json.Marshal(jsonMeta)
	node.MetaStore[namespace] = string(bytes)
	return err
}

func (node *Node) GetStringMeta(namespace string) string {
	var value string
	node.GetMeta(namespace, &value)
	return value
}

// SetMetaString sets a metadata in string format
func (node *Node) setMetaString(namespace string, meta string) {
	if node.MetaStore == nil {
		node.MetaStore = make(map[string]string)
	}
	if meta == "" {
		delete(node.MetaStore, namespace)
	} else {
		node.MetaStore[namespace] = meta
	}
}

// GetMetaString gets a metadata string
func (node *Node) getMetaString(namespace string) (meta string) {
	if node.MetaStore == nil {
		return ""
	}
	var ok bool
	if meta, ok = node.MetaStore[namespace]; ok {
		return meta
	}
	return ""
}

func (node *Node) HasMetaKey(keyName string) bool {
	if node.MetaStore == nil {
		return false
	}
	_, ok := node.MetaStore[keyName]
	return ok
}

func (node *Node) AllMetaDeserialized() map[string]interface{} {

	if len(node.MetaStore) == 0 {
		return map[string]interface{}{}
	}
	m := make(map[string]interface{}, len(node.MetaStore))
	for k, _ := range node.MetaStore {
		if strings.HasPrefix(k, "pydio:") {
			continue
		}
		var data interface{}
		node.GetMeta(k, &data)
		m[k] = data
	}
	return m

}

type jsonMarshallableNode struct {
	Node
	Meta         map[string]interface{} `json:"MetaStore"`
	ReadableType string                 `json:"type"`
}

// Specific JSON Marshalling for backward compatibility with previous Pydio
// versions
func (node *Node) MarshalJSONPB(marshaler *jsonpb.Marshaler) ([]byte, error) {

	meta := node.AllMetaDeserialized()
	node.LegacyMeta(meta)
	output := &jsonMarshallableNode{Node: *node}
	output.Meta = meta
	output.MetaStore = nil
	if node.Type == NodeType_LEAF {
		output.ReadableType = "LEAF"
		meta["is_file"] = true
	} else if node.Type == NodeType_COLLECTION {
		output.ReadableType = "COLLECTION"
		meta["is_file"] = false
	}
	return json.Marshal(output)

}

func (node *Node) LegacyMeta(meta map[string]interface{}) {
	meta["uuid"] = node.Uuid
	meta["bytesize"] = node.Size
	meta["ajxp_modiftime"] = node.MTime
	meta["etag"] = node.Etag
	if _, basename := path.Split(node.Path); basename != node.GetStringMeta("name") {
		meta["text"] = node.GetStringMeta("name")
	}
}

// HasSource checks if node has a DataSource and Object Service metadata set
func (node *Node) HasSource() bool {
	return node.HasMetaKey(common.META_NAMESPACE_DATASOURCE_NAME) && node.HasMetaKey(common.META_NAMESPACE_OBJECT_SERVICE)
}

// ReadFile opens a reader on an s3 object by reading its url from metadata
func (node *Node) ReadFile(ctx context.Context) (io.ReadCloser, error) {

	if !node.IsLeaf() {
		return nil, errors.BadRequest(common.SERVICE_TREE, "Cannot serve a folder as a file!")
	}

	// Special case for unit tests, pass a temporary folder
	// containing a file with same basename
	// Just read that file!
	if localFolder := node.GetStringMeta(common.META_NAMESPACE_NODE_TEST_LOCAL_FOLDER); localFolder != "" {

		baseName := node.GetStringMeta("name")
		targetFileName := filepath.Join(localFolder, baseName)
		return os.Open(targetFileName)

	}

	mc, bucket, e := node.getObjectClient()
	if e != nil {
		return nil, e
	}
	if meta, mOk := metadata.FromContext(ctx); mOk {
		if user, uOk := meta["x-pydio-user"]; uOk {
			mc.PrepareMetadata(map[string]string{
				"x-pydio-user": user,
			})
			defer mc.ClearMetadata()
		}
	}

	// Check that object exists

	var objectPath string
	node.GetMeta(common.META_NAMESPACE_DATASOURCE_PATH, &objectPath)
	if objectPath == "" {
		return nil, errors.BadRequest(common.SERVICE_TREE, "Empty DataSource Path in Metadata", node)
	}
	objectPath = strings.TrimLeft(objectPath, "/")

	_, oie := mc.StatObject(bucket, objectPath, minio.StatObjectOptions{})
	if oie != nil {
		log.Logger(ctx).Error("Error while reading file", zap.String("bucket", bucket), zap.String("path", objectPath), zap.Error(oie))
		return nil, oie
	}

	object, _, oe := mc.GetObject(bucket, objectPath, minio.GetObjectOptions{})
	if oe != nil {
		return nil, oe
	}

	return object, nil

}

func (node *Node) getObjectClient() (*minio.Core, string, error) {

	if !node.HasSource() {
		return nil, "", errors.BadRequest(common.SERVICE_TREE, "Node does not provide any metadata for reading content")
	}
	var dsName, endpoint string
	node.GetMeta(common.META_NAMESPACE_DATASOURCE_NAME, &dsName)
	node.GetMeta(common.META_NAMESPACE_OBJECT_SERVICE, &endpoint)

	url, bucket := filepath.Split(endpoint)

	mc, e := minio.NewCore(url, dsName, dsName+"secret", false)

	if e != nil {
		return nil, "", e
	}

	return mc, bucket, nil

}
