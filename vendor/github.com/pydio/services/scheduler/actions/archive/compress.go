package archive


import (
	"github.com/pydio/services/common/proto/jobs"
	"github.com/micro/go-micro/client"
	"golang.org/x/net/context"
	"github.com/pydio/services/common/views"
	"encoding/json"
	"github.com/pydio/services/common/log"
	"path/filepath"
	"github.com/pydio/services/common/proto/tree"
	"io"
	"go.uber.org/zap"
)

var(
	compressActionName = "actions.archive.compress"
)


type CompressAction struct{
	Router *views.Router
	Format string
	TargetName string
}

// Unique identifier
func (c *CompressAction) GetName() string {
	return compressActionName
}

// Pass parameters
func (c *CompressAction) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error{
	c.Router = views.NewStandardRouter(true, false)
	if format, ok := action.Parameters["format"]; ok {
		c.Format = format
	} else {
		c.Format = "zip"
	}
	if target, ok := action.Parameters["target"]; ok {
		c.TargetName = target
	}
	return nil
}

// Run the actual action code
func (c *CompressAction) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	if len(input.Nodes) == 0 {
		return input.WithIgnore(), nil
	}
	nodes := input.Nodes
	log.Logger(ctx).Debug("Compress to : " + c.Format)

	// Assume Target is root node sibling
	compressor := &views.ArchiveWriter{
		Router: c.Router,
	}
	if c.TargetName == "" {

	}

	base := "Archive"
	if len(nodes) == 1 {
		base = filepath.Base(nodes[0].Path)
	}
	targetFile := computeTargetName(ctx, c.Router, filepath.Dir(nodes[0].Path), base, c.Format)

	reader, writer := io.Pipe()

	var written int64
	var err error

	go func(){
		defer writer.Close()
		if c.Format == "zip" {
			written, err = compressor.ZipSelection(ctx, writer, input.Nodes)
		} else if c.Format == "tar" {
			written, err = compressor.TarSelection(ctx, writer, false, input.Nodes)
		} else if c.Format == "tar.gz" {
			written, err = compressor.TarSelection(ctx, writer, true, input.Nodes)
		}
	}()

	c.Router.PutObject(ctx, &tree.Node{Path: targetFile}, reader, &views.PutRequestData{Size:-1})

	if err != nil {
		log.Logger(ctx).Error("Error PutObject", zap.Error(err))
		return input.WithError(err), err
	}

	log, _ := json.Marshal(map[string]interface{}{
		"Written": written,
	})

	// Reload node
	resp, err := c.Router.ReadNode(ctx, &tree.ReadNodeRequest{&tree.Node{Path: targetFile}})
	if err == nil {
		input = input.WithNode(resp.Node)
		input.AppendOutput(&jobs.ActionOutput{
			Success:true,
			JsonBody:log,
		})
	} else {
		input = input.WithError(err)
	}
	return input, nil

}
