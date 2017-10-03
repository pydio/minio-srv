package dav

import (
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/pydio/services/common/log"

	"bytes"

	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/views"
	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
)

type FileSystem struct {
	mu     sync.Mutex
	Debug  bool
	Router *views.Router
}

type FileInfo struct {
	node *tree.Node
	ctx  context.Context
}

func (fi *FileInfo) Name() string {
	if fi.node.Path != "" {
		return path.Base(fi.node.Path)
	} else {
		return fi.node.GetStringMeta("name")
	}
}
func (fi *FileInfo) Size() int64 { return fi.node.Size }
func (fi *FileInfo) Mode() os.FileMode {
	mode := os.FileMode(fi.node.GetMode())
	if fi.node.Type == tree.NodeType_COLLECTION {
		mode = mode | os.ModeDir
	}
	return mode
}
func (fi *FileInfo) ModTime() time.Time { return fi.node.GetModTime() }
func (fi *FileInfo) IsDir() bool        { return !fi.node.IsLeaf() }
func (fi *FileInfo) Sys() interface{}   { return nil }

type File struct {
	fs       *FileSystem
	node     *tree.Node
	ctx      context.Context
	name     string
	off      int64
	children []os.FileInfo
}

func clearName(name string) (string, error) {
	slashed := strings.HasSuffix(name, "/")
	name = path.Clean(name)
	if !strings.HasSuffix(name, "/") && slashed {
		name += "/"
	}
	if !strings.HasPrefix(name, "/") {
		return "", os.ErrInvalid
	}
	return name, nil
}

func (fs *FileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.Debug {
		log.Logger(ctx).Debug("FileSystem.Mkdir", zap.String("name", name))
	}

	if strings.HasPrefix(path.Base(name), ".") {
		return errors.Forbidden("DAV", "Cannot create hidden files")
	}
	if !strings.HasSuffix(name, "/") {
		name += "/"
	}

	var err error
	if name, err = clearName(name); err != nil {
		return err
	}

	_, err = fs.stat(ctx, name)
	if err == nil {
		return os.ErrExist
	}

	base := ""
	for _, elem := range strings.Split(strings.Trim(name, "/"), "/") {
		base += "/" + elem
		_, err := fs.stat(ctx, base)
		if err == nil {
			continue
		}
		_, err = fs.Router.CreateNode(ctx, &tree.CreateNodeRequest{Node: &tree.Node{
			Path: base,
			Mode: int32(perm.Perm() | os.ModeDir),
			Type: tree.NodeType_COLLECTION,
		}})
		if err != nil {
			return err
		}
	}
	return nil
}

func (fs *FileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.Debug {
		log.Logger(ctx).Debug("FileSystem.OpenFile", zap.String("name", name))
	}

	var err error
	if name, err = clearName(name); err != nil {
		return nil, err
	}

	if flag&os.O_CREATE != 0 {
		// file should not have / suffix.
		if strings.HasSuffix(name, "/") {
			return nil, os.ErrInvalid
		}
		// based directory should be exists.
		dir, _ := path.Split(name)
		_, err := fs.stat(ctx, dir)
		if err != nil {
			return nil, os.ErrInvalid
		}
		var node *tree.Node
		readResp, err := fs.Router.ReadNode(ctx, &tree.ReadNodeRequest{Node: &tree.Node{Path: name}})
		if err == nil {
			if flag&os.O_EXCL != 0 {
				return nil, os.ErrExist
			}
			node = readResp.Node
		} else {
			if strings.HasPrefix(path.Base(name), ".") {
				return nil, os.ErrPermission
			}
			createNodeResponse, createErr := fs.Router.CreateNode(ctx, &tree.CreateNodeRequest{Node: &tree.Node{
				Path: name,
				Mode: 0777,
				Type: tree.NodeType_LEAF,
			}})
			if createErr != nil {
				return &File{}, createErr
			}
			node = createNodeResponse.Node
		}
		return &File{fs: fs, node: node, name: name, off: 0, children: nil, ctx: ctx}, nil
	}

	var node *tree.Node
	readResp, err := fs.Router.ReadNode(ctx, &tree.ReadNodeRequest{Node: &tree.Node{Path: name}})
	if err == nil {
		node = readResp.Node
	} else {
		createNodeResponse, _ := fs.Router.CreateNode(ctx, &tree.CreateNodeRequest{Node: &tree.Node{
			Path: name,
			Mode: 0777,
			Type: tree.NodeType_LEAF,
		}})
		node = createNodeResponse.Node
	}
	return &File{fs: fs, node: node, name: name, off: 0, children: nil, ctx: ctx}, nil

}

func (fs *FileSystem) removeAll(ctx context.Context, name string) error {
	var err error
	if name, err = clearName(name); err != nil {
		return err
	}

	fi, err := fs.stat(ctx, name)
	if err != nil {
		return err
	}
	node := fi.(*FileInfo).node
	_, err = fs.Router.DeleteNode(ctx, &tree.DeleteNodeRequest{Node: node})
	return err
}

func (fs *FileSystem) RemoveAll(ctx context.Context, name string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.Debug {
		log.Logger(ctx).Debug("FileSystem.RemoveAll", zap.String("name", name))
	}

	return fs.removeAll(ctx, name)
}

func (fs *FileSystem) Rename(ctx context.Context, oldName, newName string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.Debug {
		log.Logger(ctx).Debug("FileSystem.Rename", zap.String("from", oldName), zap.String("to", newName))
	}

	var err error
	if oldName, err = clearName(oldName); err != nil {
		return err
	}
	if newName, err = clearName(newName); err != nil {
		return err
	}

	of, err := fs.stat(ctx, oldName)
	if err != nil {
		return os.ErrExist
	}
	if of.IsDir() && !strings.HasSuffix(oldName, "/") {
		oldName += "/"
		newName += "/"
	}

	_, err = fs.stat(ctx, newName)
	if err == nil {
		return os.ErrExist
	}

	//_, err = fs.db.Exec(`update filesystem set name = ? where name = ?`, newName, oldName)
	fromNode := of.(*FileInfo).node
	_, err = fs.Router.UpdateNode(ctx, &tree.UpdateNodeRequest{From: fromNode, To: &tree.Node{Path: newName}})
	return err
}

func (fs *FileSystem) stat(ctx context.Context, name string) (os.FileInfo, error) {
	var err error
	if name, err = clearName(name); err != nil {
		log.Logger(ctx).Error("Clean Error", zap.Error(err))
		return nil, err
	}

	response, err := fs.Router.ReadNode(ctx, &tree.ReadNodeRequest{Node: &tree.Node{
		Path: name,
	}})
	if err != nil {
		log.Logger(ctx).Error("ReadNode Error", zap.Error(err))
		return nil, err
	}

	node := response.Node
	fi := &FileInfo{
		node: node,
		ctx:  ctx,
	}
	return fi, nil
}

func (fs *FileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.Debug {
		log.Logger(ctx).Debug("FileSystem.Stat", zap.String("name", name))
	}

	return fs.stat(ctx, name)
}

func (f *File) Write(p []byte) (int, error) {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()

	read, err := f.fs.Router.PutObject(f.ctx, f.node, bytes.NewBuffer(p), &views.PutRequestData{
		Size: int64(len(p)),
	})
	if err != nil {
		return 0, err
	}
	return int(read), err
}

func (f *File) Close() error {
	return nil
}

func (f *File) Read(p []byte) (int, error) {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()

	reader, err := f.fs.Router.GetObject(f.ctx, f.node, &views.GetRequestData{StartOffset: f.off, Length: int64(len(p))})
	if err != nil {
		return 0, err
	}
	length, err := reader.Read(p)
	f.off += int64(length)
	if length == 0 {
		return 0, io.EOF
	}
	return length, err
}

func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()

	if f.children == nil {

		nodesClient, err := f.fs.Router.ListNodes(f.ctx, &tree.ListNodesRequest{Node: f.node})
		if err != nil {
			return nil, err
		}

		defer nodesClient.Close()

		f.children = []os.FileInfo{}
		for {

			resp, err := nodesClient.Recv()
			if resp == nil || err != nil {
				break
			}
			f.children = append(f.children, &FileInfo{node: resp.Node})
		}
	}

	old := f.off
	if old >= int64(len(f.children)) {
		if count > 0 {
			return nil, io.EOF
		}
		return nil, nil
	}
	if count > 0 {
		f.off += int64(count)
		if f.off > int64(len(f.children)) {
			f.off = int64(len(f.children))
		}
	} else {
		f.off = int64(len(f.children))
		old = 0
	}
	return f.children[old:f.off], nil
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()

	var err error
	switch whence {
	case 0:
		f.off = 0
	case 2:
		if fi, err := f.fs.stat(f.ctx, f.name); err != nil {
			return 0, err
		} else {
			f.off = fi.Size()
		}
	}
	f.off += offset
	return f.off, err
}

func (f *File) Stat() (os.FileInfo, error) {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()

	return f.fs.stat(f.ctx, f.name)
}
