/*
 * Copyright 2007-2017 Abstrium <contact (at) pydio.com>
 * This file is part of Pydio.
 *
 * Pydio is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Pydio is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with Pydio.  If not, see <http://www.gnu.org/licenses/>.
 *
 * The latest code can be found at <https://pydio.com/>.
 */
package index

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"

	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service/context"
	"github.com/pydio/services/data/source/index/utils"

	"crypto/md5"

	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/rogpeppe/fastuuid"
	"go.uber.org/zap"
)

var (
	inserting atomic.Value
	cond      *sync.Cond
)

// TreeServer definition
type TreeServer struct {
	S3URL          string
	DataSourceName string
	client         client.Client

	rand1 *fastuuid.Generator
	rand2 *fastuuid.Generator
	rand3 *fastuuid.Generator
	rand4 *fastuuid.Generator
}

/* =============================================================================
 *  Server public Methods
 * ============================================================================ */

func init() {
	inserting.Store(make(map[string]bool))
	cond = sync.NewCond(&sync.Mutex{})
}

// NewTreeServer factory
func NewTreeServer(s3URL string, dsn string) *TreeServer {

	var rand1, rand2, rand3, rand4 *fastuuid.Generator
	var err error

	if rand1, err = fastuuid.NewGenerator(); err != nil {
		return nil
	}

	if rand2, err = fastuuid.NewGenerator(); err != nil {
		return nil
	}

	if rand3, err = fastuuid.NewGenerator(); err != nil {
		return nil
	}

	if rand4, err = fastuuid.NewGenerator(); err != nil {
		return nil
	}

	return &TreeServer{
		S3URL:          s3URL,
		DataSourceName: dsn,
		client:         client.NewClient(),
		rand1:          rand1,
		rand2:          rand2,
		rand3:          rand3,
		rand4:          rand4,
	}
}

// CreateNode implementation for the TreeServer
func (s *TreeServer) CreateNode(ctx context.Context, req *tree.CreateNodeRequest, resp *tree.CreateNodeResponse) error {


	dao := servicecontext.GetDAO(ctx).(DAO)

	var node *utils.TreeNode
	var err error

	// Checking if we have a node with the same uuid
	reqUUID := req.GetNode().GetUuid()
	update := req.GetUpdateIfExists()
	log.Logger(ctx).Info("CreateNode", zap.Any("request", req))
	if reqUUID != "" {
		if node, err = dao.GetNodeByUUID(reqUUID); err != nil {
			return errors.Forbidden(common.SERVICE_INDEX_, "Could not retrieve by uuid", err)
		} else if node != nil && update {
			if err = dao.DelNode(node); err != nil {
				return errors.Forbidden(common.SERVICE_INDEX_, "Could not replace previous node", err)
			}
		} else if node != nil {
			return errors.New(common.SERVICE_INDEX_, fmt.Sprintf("A node with same UUID already exists. Pass updateIfExists parameter if you are sure to override. %v", err), 409)
		}
	}

	// Checking if we have a node with the same path
	reqPath := safePath(req.GetNode().GetPath())
	path, created, err := s.path(ctx, reqPath, true, req.GetNode())
	if err != nil {
		return errors.InternalServerError(common.SERVICE_INDEX_, "Error while inserting node", err)
	}

	if !created {
		if update {
			node = utils.NewTreeNode()
			node.SetMPath(path...)

			if err = dao.DelNode(node); err != nil {
				return errors.Forbidden(common.SERVICE_INDEX_, "Could not replace previous node", err)
			}

			_, _, err = s.path(ctx, reqPath, true, req.GetNode())
			if err != nil {
				return errors.InternalServerError(common.SERVICE_INDEX_, "Error while inserting node", err)
			}
		} else {
			return errors.Forbidden(common.SERVICE_INDEX_, "Node path already in use", 401)
		}
	}

	node, err = dao.GetNode(path)
	if err != nil || node == nil {
		return fmt.Errorf("Could not retrieve node %s", reqPath)
	}

	// Updating Parent Nodes in Batch
	b := dao.SetNodes(req.GetNode().GetEtag(), int(req.GetNode().GetSize()))
	mpath := node.MPath.Parent()
	for len(mpath) > 0 {
		parent := utils.NewTreeNode()
		parent.SetMPath(mpath...)
		b.Send(parent)
		mpath = mpath.Parent()
	}

	if err := b.Close(); err != nil {
		return fmt.Errorf("Could not update parent nodes of %s", reqPath)
	}

	node.SetMeta(common.META_NAMESPACE_DATASOURCE_NAME, s.DataSourceName)
	node.Path = reqPath

	resp.Success = true
	resp.Node = node.Node

	s.notify(ctx, &tree.NodeChangeEvent{
		Type:   tree.NodeChangeEvent_CREATE,
		Target: node.Node,
	})

	return nil
}

// ReadNode implementation for the TreeServer
func (s *TreeServer) ReadNode(ctx context.Context, req *tree.ReadNodeRequest, resp *tree.ReadNodeResponse) error {

	log.Logger(ctx).Info("ReadNode")

	dao := servicecontext.GetDAO(ctx).(DAO)

	var node *utils.TreeNode
	var err error

	if req.GetNode().GetPath() == "" && req.GetNode().GetUuid() != "" {

		node, err = dao.GetNodeByUUID(req.GetNode().GetUuid())
		if err != nil || node == nil {
			return errors.NotFound(common.SERVICE_INDEX_, "Could not find node by Uuid "+req.GetNode().GetUuid(), 404)
		}

		// In the case we've retrieve the node by uuid, we need to retrieve the path
		var path []string
		for pnode := range dao.GetNodes(node.MPath.Parents()...) {
			path = append(path, pnode.Name())
		}
		path = append(path, node.Name())
		node.Path = safePath(strings.Join(path, "/"))

	} else {
		reqPath := safePath(req.GetNode().GetPath())

		path, _, err := s.path(ctx, reqPath, false)
		if err != nil {
			return errors.InternalServerError(common.SERVICE_INDEX_, "Error while retrieving path"+reqPath, err)
		}
		if path == nil {
			//return errors.New("Could not retrieve file path")
			// Do not return error, or send a file not exists?
			return errors.NotFound(common.SERVICE_INDEX_, "Could not retrieve node "+reqPath, 404)
		}
		node, err = dao.GetNode(path)
		if err != nil {
			return errors.NotFound(common.SERVICE_INDEX_, "Could not retrieve node "+reqPath, 404)
		}

		node.Path = reqPath
	}

	resp.Success = true

	node.SetMeta(common.META_NAMESPACE_DATASOURCE_NAME, s.DataSourceName)
	resp.Node = node.Node

	return nil
}

// ListNodes implementation for the TreeServer
func (s *TreeServer) ListNodes(ctx context.Context, req *tree.ListNodesRequest, resp tree.NodeProvider_ListNodesStream) error {

	log.Logger(ctx).Info("ListNodes")

	dao := servicecontext.GetDAO(ctx).(DAO)

	defer resp.Close()

	if req.Ancestors && req.Recursive {
		return errors.InternalServerError(common.SERVICE_INDEX_, "Please use either Recursive (children) or Ancestors (parents) flag, but not both.")
	}

	var c chan *utils.TreeNode

	// Special case for  "Ancestors", node can have either Path or Uuid
	if req.Ancestors {

		var node *utils.TreeNode
		var err error
		if req.GetNode().GetPath() == "" && req.GetNode().GetUuid() != "" {

			node, err = dao.GetNodeByUUID(req.GetNode().GetUuid())
			if err != nil {
				return errors.NotFound(common.SERVICE_INDEX_, "Could not find node by Uuid "+req.GetNode().GetUuid(), 404)
			}

		} else {

			reqPath := safePath(req.GetNode().GetPath())
			path, _, err := s.path(ctx, reqPath, false)
			if err != nil {
				return errors.InternalServerError(common.SERVICE_INDEX_, "Error while retrieving path "+reqPath, err)
			}
			if path == nil {
				return errors.NotFound(common.SERVICE_INDEX_, "Could not retrieve node "+reqPath)
			}
			node, err = dao.GetNode(path)
			if err != nil {
				return errors.InternalServerError(common.SERVICE_INDEX_, "Error while retrieving node for path "+reqPath, err)
			}

		}

		// Get Ancestors tree and rebuild pathes for each
		var path []string
		nodes := []*utils.TreeNode{}
		for pnode := range dao.GetNodes(node.MPath.Parents()...) {
			path = append(path, pnode.Name())
			pnode.Path = safePath(strings.Join(path, "/"))
			nodes = append(nodes, pnode)
		}
		// Now Reverse Slice
		last := len(nodes) - 1
		for i := 0; i < len(nodes)/2; i++ {
			nodes[i], nodes[last-i] = nodes[last-i], nodes[i]
		}
		for _, n := range nodes {
			fmt.Println(n.Node)
			resp.Send(&tree.ListNodesResponse{Node: n.Node})
		}

	} else {

		reqPath := safePath(req.GetNode().GetPath())

		path, _, err := s.path(ctx, reqPath, false)
		if err != nil {
			return errors.InternalServerError(common.SERVICE_INDEX_, "Error while retrieving path"+reqPath, err)
		}

		if path == nil {
			return errors.NotFound(common.SERVICE_INDEX_, "Could not retrieve node "+reqPath, 404)
		}

		if req.Recursive {
			c = dao.GetNodeTree(path)
		} else {
			c = dao.GetNodeChildren(path)
		}

		names := strings.Split(reqPath, "/")

		for node := range c {

			if req.FilterType != tree.NodeType_UNKNOWN && req.FilterType != node.Type {
				continue
			}
			if req.Recursive && node.Path == reqPath {
				continue
			}

			if node.Level > cap(names) {
				newNames := make([]string, len(names), node.Level)
				copy(newNames, names)
				names = newNames
			}

			names = names[0:node.Level]
			names[node.Level-1] = node.Name()

			node.Path = safePath(strings.Join(names, "/"))

			node.SetMeta(common.META_NAMESPACE_DATASOURCE_NAME, s.DataSourceName)
			resp.Send(&tree.ListNodesResponse{Node: node.Node})
		}

	}

	return nil
}

// UpdateNode implementation for the TreeServer
func (s *TreeServer) UpdateNode(ctx context.Context, req *tree.UpdateNodeRequest, resp *tree.UpdateNodeResponse) (err error) {

	log.Logger(ctx).Info("UpdateNode")

	dao := servicecontext.GetDAO(ctx).(DAO)

	reqFromPath := safePath(req.GetFrom().GetPath())
	reqToPath := safePath(req.GetTo().GetPath())

	var pathFrom, pathTo utils.MPath
	var nodeFrom, nodeTo *utils.TreeNode

	if pathFrom, _, err = s.path(ctx, reqFromPath, false); err != nil {
		return errors.InternalServerError(common.SERVICE_INDEX_, "Error while creating target path"+reqToPath, err)
	}

	if pathTo, _, err = s.path(ctx, reqToPath, true); err != nil {
		return errors.InternalServerError(common.SERVICE_INDEX_, "Error while creating target path"+reqToPath, err)
	}

	if pathFrom == nil {
		return errors.NotFound(common.SERVICE_INDEX_, "Could not retrieve node "+req.From.Path, 404)
	}

	if nodeFrom, err = dao.GetNode(pathFrom); err != nil {
		return errors.NotFound(common.SERVICE_INDEX_, "Could not retrieve node "+req.From.Path, 404)
	}

	if nodeTo, err = dao.GetNode(pathTo); err != nil {
		return errors.NotFound(common.SERVICE_INDEX_, "Could not retrieve node "+req.From.Path, 404)
	}

	// First of all, we delete the existing node
	if nodeTo != nil {
		if err = dao.DelNode(nodeTo); err != nil {
			return errors.InternalServerError(common.SERVICE_INDEX_, "Could not delete node "+req.To.Path, 404)
		}
	}

	p := pathFrom.Parent()
	pf0, psf0, pf1, psf1 := utils.NewRat(), utils.NewRat(), utils.NewRat(), utils.NewRat()
	pf0.SetMPath(p...)
	psf0.SetMPath(p.Sibling()...)
	pf1.SetMPath(pathTo.Parent()...)
	psf1.SetMPath(pathTo.Parent().Sibling()...)

	var idx uint64
	m, n := new(big.Int), new(big.Int)

	if idx, err = dao.GetNodeFirstAvailableChildIndex(pathTo.Parent()); err != nil {
		return errors.InternalServerError(common.SERVICE_INDEX_, "Could not retrieve new materialized path "+req.To.Path, 404)
	}

	m.SetUint64(idx)
	n.SetUint64(uint64(pathFrom[len(pathFrom)-1]))

	p0 := utils.NewMatrix(pf0.Num(), psf0.Num(), pf0.Denom(), psf0.Denom())
	p1 := utils.NewMatrix(pf1.Num(), psf1.Num(), pf1.Denom(), psf1.Denom())

	update := func(wg *sync.WaitGroup, node *utils.TreeNode) error {
		wg.Add(1)
		defer wg.Done()

		M0 := utils.NewMatrix(node.NV(), node.SNV(), node.DV(), node.SDV())
		M1 := utils.MoveSubtree(p0, m, p1, n, M0)

		rat := utils.NewRat()
		rat.SetFrac(M1.GetA11(), M1.GetA12())

		node.SetRat(rat)

		filenames := strings.Split(reqToPath, "/")

		// We only update the node name for the root node
		// Checking the level vs the filenames is one way to check we're at the root
		if node.Level <= len(filenames) {
			node.SetMeta("name", filenames[node.Level-1])
		}

		return dao.SetNode(node)
	}

	// Updating the orginal node
	wg := &sync.WaitGroup{}

	go update(wg, nodeFrom)

	for node := range dao.GetNodeTree(pathFrom) {
		go update(wg, node)
	}

	wg.Wait()

	// Updating parents size if needed
	if dirWithInternalSeparator(reqFromPath) != dirWithInternalSeparator(reqToPath) && nodeFrom.GetSize() > 0 {
		// Updating Parent Nodes in Batch
		bFrom := dao.SetNodes(nodeFrom.GetEtag(), int(-nodeFrom.GetSize()))

		mpathFrom := pathFrom.Parent()
		for len(mpathFrom) > 0 {
			node := utils.NewTreeNode()
			node.SetMPath(mpathFrom...)
			bFrom.Send(node)
			mpathFrom = mpathFrom.Parent()
		}

		if err := bFrom.Close(); err != nil {
			return errors.InternalServerError(common.SERVICE_INDEX_, fmt.Sprintf("Failed to update some or all of the nodes from %s to %s", req.From.Path, req.To.Path), 500)
		}

		// Updating Parent Nodes in Batch
		bTo := dao.SetNodes(nodeFrom.GetEtag(), int(nodeFrom.GetSize()))

		mpathTo := pathTo.Parent()
		for len(mpathTo) > 0 {
			node := utils.NewTreeNode()
			node.SetMPath(mpathTo...)
			bTo.Send(node)
			mpathTo = mpathTo.Parent()
		}

		if err := bTo.Close(); err != nil {
			return errors.InternalServerError(common.SERVICE_INDEX_, fmt.Sprintf("Failed to update some or all of the nodes from %s to %s", req.From.Path, req.To.Path), 500)
		}
	}

	resp.Success = true

	go func() {
		newNode, err := dao.GetNode(pathTo)
		if err == nil && newNode != nil {
			newNode.Path = reqToPath
			s.notify(ctx, &tree.NodeChangeEvent{
				Type:   tree.NodeChangeEvent_UPDATE_PATH,
				Source: req.From,
				Target: newNode.Node,
			})
		}
	}()

	return nil
}

// DeleteNode implementation for the TreeServer
func (s *TreeServer) DeleteNode(ctx context.Context, req *tree.DeleteNodeRequest, resp *tree.DeleteNodeResponse) error {

	log.Logger(ctx).Info("DeleteNode")

	dao := servicecontext.GetDAO(ctx).(DAO)

	reqPath := safePath(req.GetNode().GetPath())

	path, _, _ := s.path(ctx, reqPath, false)
	if path == nil {
		return errors.NotFound(common.SERVICE_INDEX_, "Could not retrieve node "+reqPath, 404)
	}

	node, err := dao.GetNode(path)
	if err != nil {
		return errors.NotFound(common.SERVICE_INDEX_, "Could not retrieve node "+reqPath, 404)
	}
	node.Path = reqPath

	if err := dao.DelNode(node); err != nil {
		return errors.InternalServerError(common.SERVICE_INDEX_, "Could not delete node "+reqPath, 404)
	}

	if node.Size > 0 {

		// Updating Parent Nodes in Batch
		b := dao.SetNodes(node.Etag, int(-node.Size))

		mpath := node.MPath.Parent()
		for len(mpath) > 0 {
			parent := utils.NewTreeNode()
			parent.SetMPath(mpath...)
			b.Send(parent)
			mpath = mpath.Parent()
		}

		if err := b.Close(); err != nil {
			return errors.InternalServerError(common.SERVICE_INDEX_, "Could not delete node "+reqPath, 404)
		}
	}

	resp.Success = true

	s.notify(ctx, &tree.NodeChangeEvent{
		Type:   tree.NodeChangeEvent_DELETE,
		Source: node.Node,
	})

	return nil
}

func (s *TreeServer) path(ctx context.Context, strpath string, create bool, reqNode ...*tree.Node) (utils.MPath, bool, error) {
	dao := servicecontext.GetDAO(ctx).(DAO)

	var path utils.MPath
	var created = create
	var err error

	if len(strpath) == 0 || strpath == "/" {
		return []uint64{1}, false, nil
	}

	names := strings.Split(fmt.Sprintf("/%s", strings.TrimLeft(strpath, "/")), "/")

	path = make([]uint64, len(names))
	path[0] = 1
	parents := make([]*utils.TreeNode, len(names))

	// Reading root path
	node, err := dao.GetNode(path[0:1])
	if err != nil || node == nil {
		// Making sure we have a node in the database
		node = NewNode(&tree.Node{
			Uuid: "ROOT",
			Type: tree.NodeType_COLLECTION,
		}, []uint64{1}, []string{""})

		if err = dao.AddNode(node); err != nil {
			return path, false, err
		}
	}

	parents[0] = node

	maxLevel := len(names) - 1

	for level := 1; level <= maxLevel; level++ {

		p := node

		if create {
			// Making sure we lock the parent node
			cond.L.Lock()
			for {
				current := inserting.Load().(map[string]bool)

				if _, ok := current[p.Uuid]; !ok {
					current[p.Uuid] = true
					inserting.Store(current)
					break
				}

				cond.Wait()
			}
			cond.L.Unlock()
		}

		node, _ = dao.GetNodeChild(path[0:level], names[level])

		if nil != node {
			if level == maxLevel {
				created = false
			}

			res := new(big.Int)

			res.Sub(node.NV(), p.NV())
			res.Div(res, p.SNV())
			path[level] = res.Uint64()
			parents[level] = node

			node.Path = strings.Trim(strings.Join(names[0:level], string(os.PathSeparator)), string(os.PathSeparator))
		} else {
			if create {
				if path[level], err = dao.GetNodeFirstAvailableChildIndex(path[0:level]); err != nil {
					return nil, false, err
				}

				if level == len(names)-1 && len(reqNode) > 0 {
					node = NewNode(reqNode[0], path[0:level+1], names[0:level+1])
				} else {
					node = NewNode(&tree.Node{
						Type:  tree.NodeType_COLLECTION,
						Mode:  0777,
						MTime: time.Now().Unix(),
					}, path[0:level+1], names[0:level+1])
				}

				if node.Uuid == "" {
					random1 := s.rand1.Next()
					random2 := s.rand2.Next()
					random3 := s.rand3.Next()
					random4 := s.rand4.Next()
					uuid := fmt.Sprintf("%s-%s-%s-%s", hex.EncodeToString(random1[:])[0:8], hex.EncodeToString(random2[:])[0:8], hex.EncodeToString(random3[:])[0:8], hex.EncodeToString(random4[:])[0:8])
					node.Uuid = uuid
				}

				if node.Etag == "" {
					// Should only happen for folders - generate first Etag from uuid+mtime
					node.Etag = fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s%d", node.Uuid, node.MTime))))
				}

				err = dao.AddNode(node)

				cond.L.Lock()
				current := inserting.Load().(map[string]bool)
				delete(current, p.Uuid)
				inserting.Store(current)
				cond.L.Unlock()

				cond.Signal()

				if err != nil {
					return nil, false, err
				}

				parents[level] = node
			} else {
				return nil, false, nil
			}

		}

		if create {
			cond.L.Lock()
			current := inserting.Load().(map[string]bool)
			delete(current, p.Uuid)
			inserting.Store(current)
			cond.L.Unlock()

			cond.Signal()
		}
	}

	return path, created, err
}

func (s *TreeServer) notify(ctx context.Context, event *tree.NodeChangeEvent) {

	if event.Source != nil {
		event.Source.SetMeta(common.META_NAMESPACE_DATASOURCE_NAME, s.DataSourceName)
	}

	if event.Target != nil {
		event.Target.SetMeta(common.META_NAMESPACE_DATASOURCE_NAME, s.DataSourceName)
	}

	client.Publish(ctx, client.NewPublication(common.TOPIC_INDEX_CHANGES, event))

}

// NewNode utils
func NewNode(treeNode *tree.Node, path utils.MPath, filenames []string) *utils.TreeNode {

	node := utils.NewTreeNode()
	node.Node = treeNode
	node.SetMPath(path...)

	node.SetMeta("name", filenames[len(filenames)-1])

	node.Path = strings.Join(filenames, string(os.PathSeparator))

	return node
}

func safePath(str string) string {
	return fmt.Sprintf("/%s", strings.TrimLeft(str, "/"))
}

func dirWithInternalSeparator(filePath string) string {
	segments := strings.Split(filePath, "/")
	return strings.Join(segments[:len(segments)-1], "/")
}
