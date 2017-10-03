package archive

import (
	"github.com/pydio/services/scheduler/actions"
	"github.com/pydio/services/common/views"
	"golang.org/x/net/context"
	"fmt"
	"github.com/pydio/services/common/proto/tree"
)

func init(){

	manager := actions.GetActionsManager()

	manager.Register(compressActionName, func() actions.ConcreteAction {
		return &CompressAction{}
	})

	manager.Register(extractActionName, func() actions.ConcreteAction {
		return &ExtractAction{}
	})

}

func computeTargetName(ctx context.Context, handler views.Handler, dirPath string, base string, extension ...string) string {
	ext := ""
	if len(extension) > 0 {
		ext = "." + extension[0]
	}
	index := 0
	for{
		suffix := ""
		if index > 0 {
			suffix = fmt.Sprintf("-%d", index)
		}
		testPath := dirPath + "/" + base + suffix + ext
		if resp, err := handler.ReadNode(ctx, &tree.ReadNodeRequest{Node: &tree.Node{Path: testPath}}); err == nil && resp.Node != nil {
			// node exists, try next one
			index ++
		} else {
			return testPath
		}
	}
}