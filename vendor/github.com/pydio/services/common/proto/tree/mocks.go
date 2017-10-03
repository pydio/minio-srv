package tree

import (
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"golang.org/x/net/context"
)

type StreamerMock struct{}

func (*StreamerMock) Context() context.Context {
	return nil
}
func (*StreamerMock) Request() client.Request {
	return nil
}
func (*StreamerMock) Send(interface{}) error {
	return nil
}
func (*StreamerMock) Recv(interface{}) error {
	return nil
}
func (*StreamerMock) Error() error {
	return nil
}
func (*StreamerMock) Close() error {
	return nil
}

type NodeProviderMock struct {
	Nodes map[string]string
}

func (m *NodeProviderMock) ReadNode(ctx context.Context, in *ReadNodeRequest, opts ...client.CallOption) (*ReadNodeResponse, error) {

	if in.Node.Path != "" {
		if v, ok := m.Nodes[in.Node.Path]; ok {
			resp := &ReadNodeResponse{
				Node: &Node{Path: in.Node.Path, Uuid: v},
			}
			return resp, nil
		}
	} else if in.Node.Uuid != "" {
		// Search by Uuid
		for k, v := range m.Nodes {
			if v == in.Node.Uuid {
				return &ReadNodeResponse{
					Node: &Node{Path: k, Uuid: v},
				}, nil
			}
		}
	}
	return nil, errors.NotFound(common.SERVICE_INDEX_, "Node not found", 404)

}

func (m *NodeProviderMock) ListNodes(ctx context.Context, in *ListNodesRequest, opts ...client.CallOption) (NodeProvider_ListNodesClient, error) {

	// Create fake stream
	return &nodeProviderListNodesClient{stream: &StreamerMock{}}, nil

}

type NodeReceiverMock struct {
	Nodes map[string]string
}

func (m *NodeReceiverMock) CreateNode(ctx context.Context, in *CreateNodeRequest, opts ...client.CallOption) (*CreateNodeResponse, error) {
	return &CreateNodeResponse{Node: in.Node}, nil
}

func (m *NodeReceiverMock) UpdateNode(ctx context.Context, in *UpdateNodeRequest, opts ...client.CallOption) (*UpdateNodeResponse, error) {
	return &UpdateNodeResponse{Success: true}, nil
}

func (m *NodeReceiverMock) DeleteNode(ctx context.Context, in *DeleteNodeRequest, opts ...client.CallOption) (*DeleteNodeResponse, error) {
	return &DeleteNodeResponse{Success: true}, nil
}
