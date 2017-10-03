package common

import (
	"io"
	"strings"
	"time"

	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"github.com/micro/go-micro/metadata"
)

// EventType represents the type of the event occurred.
type EventType string

const (
	// EventCreate notifies when a new object is created
	EventCreate EventType = "ObjectCreated"
	// EventCreate notifies when a new object is created
	EventRename EventType = "ObjectRenamed"
	// EventRemove notifies when a new object is deleted
	EventRemove EventType = "ObjectRemoved"
	// EventAccessed notifies when an object is accessed.
	EventAccessed EventType = "ObjectAccessed"
	// EventAccessedRead notifies when an object is accessed (specifically read).
	EventAccessedRead EventType = "ObjectAccessed:Read"
	// EventAccessedStat notifies when an object is accessed (specifically stat).
	EventAccessedStat EventType = "ObjectAccessed:Stat"
	// EventOther notifies any other events
	EventOther EventType = "ObjectOther"

	// Use unique path separator everywhere
	InternalPathSeparator = "/"
)

/*
type Node struct {
	Path     string
	Leaf     bool
	Hash     string
	Uuid     string
	FileInfo os.FileInfo
}
*/

func AsPathSyncSource(endpoint Endpoint) (PathSyncSource, bool) {
	i, ok := interface{}(endpoint).(PathSyncSource)
	return i, ok
}
func AsPathSyncTarget(endpoint Endpoint) (PathSyncTarget, bool) {
	i, ok := interface{}(endpoint).(PathSyncTarget)
	return i, ok
}
func AsDataSyncSource(endpoint Endpoint) (DataSyncSource, bool) {
	i, ok := interface{}(endpoint).(DataSyncSource)
	return i, ok
}
func AsDataSyncTarget(endpoint Endpoint) (DataSyncTarget, bool) {
	i, ok := interface{}(endpoint).(DataSyncTarget)
	return i, ok
}
func AsVersioner(endpoint Endpoint) (Versioner, bool) {
	i, ok := interface{}(endpoint).(Versioner)
	return i, ok
}

type EndpointInfo struct {
	RequiresNormalization bool
	RequiresFoldersRescan bool
}

type Endpoint interface {
	LoadNode(ctx context.Context, path string, leaf ...bool) (node *tree.Node, err error)
	GetEndpointInfo() EndpointInfo
}

type WalkNodesFunc func(path string, node *tree.Node, err error)

type PathSyncSource interface {
	Endpoint
	Walk(walknFc WalkNodesFunc, pathes ...string) (err error)
	Watch(recursivePath string) (*WatchObject, error)
}

type PathSyncTarget interface {
	Endpoint
	CreateNode(ctx context.Context, node *tree.Node, updateIfExists bool) (err error)
	UpdateNode(ctx context.Context,node *tree.Node) (err error)
	DeleteNode(ctx context.Context,path string) (err error)
	MoveNode(ctx context.Context,oldPath string, newPath string) (err error)
}

type DataSyncTarget interface {
	PathSyncTarget
	GetWriterOn(path string) (out io.WriteCloser, err error)
}

type DataSyncSource interface {
	PathSyncSource
	GetReaderOn(path string) (out io.ReadCloser, err error)
}

type Versioner interface {
	Commit(node *tree.Node)
	ListVersions(node *tree.Node) (versions map[int]string, lastVersion int)
}

// EventInfo contains the information of the event that occurred and the source
// IP:PORT of the client which triggerred the event.
type EventInfo struct {
	Time           string
	Size           int64
	Etag           string
	Folder         bool
	Path           string
	PathSyncSource PathSyncSource
	Type           EventType
	Host           string
	Port           string
	UserAgent      string
	OperationId    string
	ScanEvent      bool
	ScanSourceNode *tree.Node
	Metadata 	   map[string]string
}

func (e EventInfo) CreateContext() context.Context {
	ctx := context.Background()
	if e.Metadata == nil {
		return ctx
	} else {
		return metadata.NewContext(ctx, e.Metadata)
	}
}

type ProcessorEvent struct {
	Type string
	Data interface{}
}

type WatchObject struct {
	// eventInfo will be put on this chan
	EventInfoChan chan EventInfo
	// errors will be put on this chan
	ErrorChan chan error
	// will stop the watcher goroutines
	DoneChan chan bool
}

// NextEvent pops the next event from the EventInfoChan
func (w WatchObject) NextEvent() EventInfo {
	return <-w.EventInfoChan
}

// NextError pops the next error from the ErrorChan
func (w WatchObject) NextError() error {
	return <-w.ErrorChan
}

// Done returns a channel that unblocks when Close has been
// called
func (w WatchObject) Done() <-chan bool { return w.DoneChan }

// Events returns the chan receiving events
func (w *WatchObject) Events() chan EventInfo {
	return w.EventInfoChan
}

// Errors returns the chan receiving errors
func (w *WatchObject) Errors() chan error {
	return w.ErrorChan
}

// Close the watcher, will stop all goroutines
func (w *WatchObject) Close() {
	close(w.DoneChan)
}

func IsIgnoredFile(path string) (ignored bool) {
	return strings.HasSuffix(path, ".DS_Store") || strings.Contains(path, ".minio.sys") || strings.HasSuffix(path, "$buckets.json") || strings.HasSuffix(path, "$multiparts-session.json") || strings.HasSuffix(path, "--COMPUTE_HASH")
}

func NodeToEventInfo(ctx context.Context, path string, node *tree.Node, eventType EventType) (eventInfo EventInfo) {
	timeFormatFS := "2006-01-02T15:04:05.000Z"
	eventInfo = EventInfo{
		Time:           time.Now().UTC().Format(timeFormatFS),
		Folder:         !node.IsLeaf(),
		Path:           path,
		Type:           eventType,
		ScanEvent:      true,
		ScanSourceNode: node,
	}
	if ctx != nil {
		if 	meta, ok := metadata.FromContext(ctx); ok {
			eventInfo.Metadata = meta
		}
	}
	return eventInfo
}

func DirWithInternalSeparator(filePath string) string {

	segments := strings.Split(filePath, InternalPathSeparator)
	return strings.Join(segments[:len(segments)-1], InternalPathSeparator)

}
