package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/views"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

var (
	wgetActionName = "actions.cmd.wget"
)

type WGetAction struct {
	Router    *views.Router
	SourceUrl *url.URL
}

// Unique identifier
func (w *WGetAction) GetName() string {
	return wgetActionName
}

// Pass parameters
func (w *WGetAction) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {
	if urlParam, ok := action.Parameters["url"]; ok {
		var e error
		w.SourceUrl, e = url.Parse(urlParam)
		if e != nil {
			return e
		}
	} else {
		return errors.BadRequest(common.SERVICE_TASKS, "Missing parameter url in Action")
	}
	w.Router = views.NewStandardRouter(true, false)
	return nil
}

// Run the actual action code
func (w *WGetAction) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	if len(input.Nodes) == 0 {
		return input.WithIgnore(), nil
	}
	targetNode := input.Nodes[0]
	log.Logger(ctx).Debug("WGET: " + w.SourceUrl.String())
	httpResponse, err := http.Get(w.SourceUrl.String())
	if err != nil {
		return input.WithError(err), err
	}
	start := time.Now()
	defer httpResponse.Body.Close()
	var written int64
	var er error
	if localFolder := targetNode.GetStringMeta(common.META_NAMESPACE_NODE_TEST_LOCAL_FOLDER); localFolder != "" {
		var localFile *os.File
		localFile, er = os.OpenFile(filepath.Join(localFolder, targetNode.Uuid), os.O_CREATE|os.O_WRONLY, 0755)
		if er == nil {
			written, er = io.Copy(localFile, httpResponse.Body)
		}
	} else {
		written, er = w.Router.PutObject(ctx, targetNode, httpResponse.Body, &views.PutRequestData{Size: httpResponse.ContentLength})
	}
	log.Logger(ctx).Debug("After PUT Object", zap.Int64("Written Bytes", written), zap.Error(er), zap.Any("ctx", ctx))
	if er != nil {
		return input.WithError(er), err
	}
	last := time.Now().Sub(start)
	log, _ := json.Marshal(map[string]interface{}{
		"Size": written,
		"Time": last,
	})
	input.AppendOutput(&jobs.ActionOutput{
		Success:  true,
		JsonBody: log,
	})
	return input, nil

}
