package views

import (
	"golang.org/x/net/context"
	"github.com/pydio/services/common/proto/tree"
	"github.com/micro/go-micro/client"
	"io"
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"
	"github.com/pydio/services/common"
)

type VersionHandler struct{
	AbstractHandler
	versionClient tree.NodeVersionerClient
}

func (v *VersionHandler) getVersionClient() tree.NodeVersionerClient{
	if v.versionClient == nil {
		v.versionClient = tree.NewNodeVersionerClient(common.SERVICE_VERSIONS, v.clientsPool.genericClient)
	}
	return v.versionClient
}

// Create list of nodes if the Versions are required
func (v *VersionHandler) ListNodes(ctx context.Context, in *tree.ListNodesRequest, opts ...client.CallOption) (tree.NodeProvider_ListNodesClient, error) {
	ctx, err := v.wrapContext(ctx)
	if err != nil {
		return nil, err
	}
	if in.Versions {

		streamer := NewWrappingStreamer()
		resp, e := v.next.ReadNode(ctx, &tree.ReadNodeRequest{Node: in.Node})
		if e != nil {
			return streamer, e
		}
		versionStream, er := v.getVersionClient().ListVersions(ctx, &tree.ListVersionsRequest{Node:resp.Node})
		if er != nil {
			return streamer, er
		}
		go func(){
			defer streamer.Close()

			log.Logger(ctx).Debug("SHOULD LIST VERSIONS OF OBJECT", zap.Any("node", resp.Node), zap.Error(er))
			for {
				vResp, vE := versionStream.Recv()
				if vE != nil {
					log.Logger(ctx).Error("Reading Stream of versions", zap.Error(vE))
					break
				}
				if vResp == nil {
					log.Logger(ctx).Debug("NIL RESP ON VERSIONS OF OBJECT", zap.Any("node", resp.Node))
					continue
				}
				log.Logger(ctx).Debug("RECEIVED VERSION", zap.Any("version", vResp))
				vNode := resp.Node
				vNode.Etag = string(vResp.Version.Data)
				vNode.MTime = vResp.Version.MTime
				vNode.Size = vResp.Version.Size
				vNode.SetMeta("versionId", vResp.Version.Uuid)
				streamer.Send(&tree.ListNodesResponse{
					Node:vNode,
				})
			}
		}()
		return streamer, nil

	} else {
		return v.next.ListNodes(ctx, in, opts...)
	}

}

func (v *VersionHandler) ReadNode(ctx context.Context, req *tree.ReadNodeRequest, opts ...client.CallOption) (*tree.ReadNodeResponse, error) {

	if vId := req.Node.GetStringMeta("versionId"); vId != "" {
		// Load Info from Version Service?
		node := req.Node
		if len(node.Uuid) == 0 {
			resp, e := v.next.ReadNode(ctx, &tree.ReadNodeRequest{Node: node})
			if e != nil {
				return nil, e
			}
			node = resp.Node
		}
		vResp, err := v.getVersionClient().HeadVersion(ctx, &tree.HeadVersionRequest{Node:node, VersionId:vId})
		if err != nil {
			return nil, err
		}
		node.Etag = string(vResp.Version.Data)
		node.MTime = vResp.Version.MTime
		node.Size = vResp.Version.Size
		return &tree.ReadNodeResponse{Node: node}, nil

	}

	return v.next.ReadNode(ctx, req, opts...)
}

// Redirect to Version Store if request contains a VersionID
func (v *VersionHandler) GetObject(ctx context.Context, node *tree.Node, requestData *GetRequestData) (io.ReadCloser, error) {
	ctx, err := v.wrapContext(ctx)
	if err != nil {
		return nil, err
	}
	if len(requestData.VersionId) > 0 {

		dsi, e := v.clientsPool.GetDataSourceInfo(common.PYDIO_VERSIONS_NAMESPACE)
		if e != nil {
			return nil, e
		}
		// We are trying to load a specific versionId => switch to vID store
		if len(node.Uuid) == 0 {
			resp, e := v.next.ReadNode(ctx, &tree.ReadNodeRequest{Node: node})
			if e != nil {
				return nil, e
			}
			node = resp.Node
		}
		node = &tree.Node{
			Path: node.Uuid + "__" + requestData.VersionId,
		}
		branchInfo := BranchInfo{DSInfo: dsi}
		ctx = WithBranchInfo(ctx, "in", branchInfo)

	}
	return v.next.GetObject(ctx, node, requestData)

}

// Read from Version Store if request contains a VersionID
func (v *VersionHandler) CopyObject(ctx context.Context, from *tree.Node, to *tree.Node, requestData *CopyRequestData) (int64, error) {
	ctx, err := v.wrapContext(ctx)
	if err != nil {
		return 0, err
	}

	if len(requestData.srcVersionId) > 0 {

		dsi, e := v.clientsPool.GetDataSourceInfo(common.PYDIO_VERSIONS_NAMESPACE)
		if e != nil {
			return 0, e
		}
		// We are trying to load a specific versionId => switch to vID store
		if len(from.Uuid) == 0 {
			resp, e := v.next.ReadNode(ctx, &tree.ReadNodeRequest{Node: from})
			if e != nil {
				return 0, e
			}
			from = resp.Node
		}
		from = &tree.Node{
			Path: from.Uuid + "__" + requestData.srcVersionId,
		}
		branchInfo := BranchInfo{DSInfo: dsi}
		ctx = WithBranchInfo(ctx, "from", branchInfo)

	}

	return v.next.CopyObject(ctx, from, to, requestData)
}

