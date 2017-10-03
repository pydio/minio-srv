package endpoints

import (
	"errors"
	"github.com/pydio/poc/sync/common"
	"golang.org/x/text/unicode/norm"
	"os"
	"strings"

	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

type DBEvent struct {
	Type   string
	Source string
	Target string
}

type MemDB struct {
	Nodes         []*tree.Node
	eventChannels []chan DBEvent
}

func (c *MemDB) GetEndpointInfo() common.EndpointInfo {

	return common.EndpointInfo{
		RequiresFoldersRescan: false,
		RequiresNormalization: false,
	}

}

/*************************/
/* Event Emitter 	 */
/*************************/
func (db *MemDB) RegisterEventChannel(out chan DBEvent) {
	db.eventChannels = append(db.eventChannels, out)
}

func (db *MemDB) sendEvent(event DBEvent) {
	for _, channel := range db.eventChannels {
		channel <- event
	}
}

/*************************/
/* Path Sync Target 	 */
/*************************/
func (db *MemDB) LoadNode(ctx context.Context, path string, leaf ...bool) (node *tree.Node, err error) {

	for _, node := range db.Nodes {
		if norm.NFC.String(node.Path) == norm.NFC.String(path) {
			return node, nil
		}
	}

	return nil, errors.New("Node not found")
}

func (db *MemDB) CreateNode(ctx context.Context, node *tree.Node, updateIfExists bool) (err error) {
	db.Nodes = append(db.Nodes, node)
	db.sendEvent(DBEvent{
		Type:   "Create",
		Target: node.Path,
	})
	return nil
}

func (db *MemDB) UpdateNode(ctx context.Context, node *tree.Node) (err error) {
	removed := db.removeNodeNoEvent(node.Path)
	db.Nodes = append(db.Nodes, node)
	if removed == nil {
		db.sendEvent(DBEvent{
			Type:   "Create",
			Target: node.Path,
		})
	} else {
		db.sendEvent(DBEvent{
			Type:   "Update",
			Source: node.Path,
			Target: node.Path,
		})
	}
	return nil
}

func (db *MemDB) DeleteNode(ctx context.Context, path string) (err error) {
	removed := db.removeNodeNoEvent(path)
	if removed != nil {
		db.sendEvent(DBEvent{
			Type:   "Delete",
			Source: removed.Path,
		})
	}
	return nil
}

func (db *MemDB) MoveNode(ctx context.Context, oldPath string, newPath string) (err error) {
	moved := false
	for _, node := range db.Nodes {
		if strings.HasPrefix(node.Path, oldPath+string(os.PathSeparator)) || node.Path == oldPath {
			node.Path = newPath + strings.TrimPrefix(node.Path, oldPath)
			moved = true
		}
	}
	if moved {
		db.sendEvent(DBEvent{
			Type:   "Moved",
			Source: oldPath,
			Target: newPath,
		})
	}
	return nil
}

/*************************/
/* Path Sync Source 	 */
/*************************/
func (db *MemDB) Walk(walknFc common.WalkNodesFunc, pathes ...string) (err error) {
	var ignore bool
	for _, node := range db.Nodes {
		if len(pathes) > 0 {
			// If there are some limitations on path, detect them
			ignore = false
			for _, testPath := range pathes {
				if !strings.HasPrefix(node.Path, testPath) {
					ignore = true
					break
				}
			}
			if ignore {
				continue
			}
		}
		walknFc(node.Path, node, nil)
	}
	return nil
}

func (db *MemDB) Watch(recursivePath string) (*common.WatchObject, error) {
	return nil, errors.New("Not implemented")
}

/*************************/
/* Other Methods 	 */
/*************************/
func (db *MemDB) removeNodeNoEvent(path string) (removed *tree.Node) {
	var newNodes []*tree.Node
	for _, node := range db.Nodes {
		if !strings.HasPrefix(node.Path, path+string(os.PathSeparator)) && node.Path != path {
			newNodes = append(newNodes, node)
		} else {
			removed = node
		}
	}
	db.Nodes = newNodes
	return removed
}

func (db *MemDB) FindByHash(hash string) (node *tree.Node) {

	for _, node := range db.Nodes {
		if node.Etag == hash {
			return node
		}
	}
	return
}

func (db *MemDB) FindByUuid(id string) (node *tree.Node) {

	for _, node := range db.Nodes {
		if node.Uuid == id {
			return node
		}
	}
	return
}

func (db *MemDB) String() string {
	output := ""
	for _, node := range db.Nodes {
		leaf := "Leaf"
		if !node.IsLeaf() {
			leaf = "Folder"
		}
		output += leaf + "\t'" + node.Path + "' (" + node.Uuid + node.Etag + ")" + "\n"
	}
	return output
}

func NewMemDB() *MemDB {
	db := &MemDB{}
	db.CreateNode(context.Background(), &tree.Node{
		Path: string(os.PathSeparator),
		Type: tree.NodeType_COLLECTION,
		Uuid: "root",
	}, true)
	return db
}
