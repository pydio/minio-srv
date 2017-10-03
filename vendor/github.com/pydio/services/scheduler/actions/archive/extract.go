package archive

import (
	"github.com/pydio/services/common/views"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/micro/go-micro/client"
	"golang.org/x/net/context"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"path/filepath"
	"strings"
	"github.com/pydio/services/common/proto/tree"
)

var (
	extractActionName = "actions.archive.extract"
)

type ExtractAction struct{
	Router *views.Router
	Format string
	TargetName string
}

// Unique identifier
func (ex *ExtractAction) GetName() string {
	return extractActionName
}

// Pass parameters
func (ex *ExtractAction) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {
	ex.Router = views.NewStandardRouter(true, false)
	if format, ok := action.Parameters["format"]; ok {
		ex.Format = format
	}
	if target, ok := action.Parameters["target"]; ok {
		ex.TargetName = target
	}
	return nil
}


// Run
func (ex *ExtractAction) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	if len(input.Nodes) == 0 {
		return input.WithIgnore(), nil
	}
	archiveNode := input.Nodes[0]
	ext := filepath.Ext(archiveNode.Path)
	if ext == ".gz" && strings.HasSuffix(archiveNode.Path, ".tar.gz") {
		ext = ".tar.gz"
	}

	format := ex.Format
	if format == "" {
		format = strings.TrimLeft(ext, ".")
	}
	targetName := ex.TargetName
	if targetName == "" {
		base := strings.TrimSuffix(filepath.Base(archiveNode.Path), ext)
		targetName = computeTargetName(ctx, ex.Router, filepath.Dir(archiveNode.Path), base)
	}
	targetNode := &tree.Node{Path: targetName, Type: tree.NodeType_COLLECTION}
	_, e := ex.Router.CreateNode(ctx, &tree.CreateNodeRequest{Node: targetNode})
	if e != nil {
		return input.WithError(e), e
	}

	reader := &views.ArchiveReader{
		Router: ex.Router,
	}
	var err error
	switch format {
	case "zip":
		err = reader.ExtractAllZip(ctx, archiveNode, targetNode)
		break
	case "tar":
		err = reader.ExtractAllTar(ctx, false, archiveNode, targetNode)
		break
	case "tar.gz":
		err = reader.ExtractAllTar(ctx, true, archiveNode, targetNode)
		break
	default:
		err = errors.BadRequest(common.SERVICE_JOBS, "Unsupported archive format:" + format)
	}
	if err != nil {
		// Remove failed extraction folder ?
		// ex.Router.DeleteNode(ctx, &tree.DeleteNodeRequest{Node: targetNode})
		return input.WithError(err), err
	}

	output := input.WithNode(targetNode)

	return output, nil
}