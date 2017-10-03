package versions

import (
	"golang.org/x/net/context"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"
)

type Handler struct{
	db DAO
}

func (h *Handler) ListVersions(ctx context.Context, request *tree.ListVersionsRequest, versionsStream tree.NodeVersioner_ListVersionsStream) error {

	log.Logger(ctx).Info("[VERSION] ListVersions for node ", zap.Any("node", request.Node))
	logs, done := h.db.GetVersions(request.Node.Uuid)

	defer versionsStream.Close()
	for {
		select{
		case l := <- logs:
			resp := &tree.ListVersionsResponse{Version:l}
			e := versionsStream.Send(resp)
			log.Logger(ctx).Debug("[VERSION] Sending version ", zap.Any("resp", resp), zap.Error(e))
			break
		case <-done:
			log.Logger(ctx).Debug("List Versions: break now")
			return nil
		}
	}

	return nil
}

func (h *Handler) HeadVersion(ctx context.Context, request *tree.HeadVersionRequest, resp *tree.HeadVersionResponse) error {

	v, e := h.db.GetVersion(request.Node.Uuid, request.VersionId)
	if e != nil {
		return e
	}
	if (v != &tree.ChangeLog {}) {
		resp.Version = v
	}
	return nil
}

func (h *Handler) CreateVersion(ctx context.Context, request *tree.CreateVersionRequest, resp *tree.CreateVersionResponse) error {

	log.Logger(ctx).Info("[VERSION] GetLastVersion for node " + request.Node.Uuid)
	last, err := h.db.GetLastVersion(request.Node.Uuid)
	if err != nil {
		return err
	}
	log.Logger(ctx).Info("[VERSION] GetLastVersion for node ", zap.Any("last", last), zap.Any("node", request.Node))
	if last == nil || string(last.Data) != request.Node.Etag {
		resp.Version = tree.NewChangeLogFromNode(request.Node)
	}
	return nil
}

func (h *Handler) StoreVersion(ctx context.Context, request *tree.StoreVersionRequest, resp *tree.StoreVersionResponse) error {

	log.Logger(ctx).Info("[VERSION] StoreVersion for node ", zap.Any("res",  request))
	err := h.db.StoreVersion(request.Node.Uuid, request.Version)
	if err == nil {
		resp.Success = true
	}
	return err

}

