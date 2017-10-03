package views

import (
	"io"

	"github.com/micro/go-plugins/client/grpc"
	"github.com/pydio/minio-go/pkg/encrypt"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

type EncryptionHandler struct {
	AbstractHandler
	EncryptionClient tree.FileKeyManagerClient
}

func (e *EncryptionHandler) getClient() tree.FileKeyManagerClient {

	if e.EncryptionClient == nil {
		e.EncryptionClient = tree.NewFileKeyManagerClient(common.SERVICE_ENCRYPTION, grpc.NewClient())
	}
	return e.EncryptionClient

}

func (e *EncryptionHandler) retrieveEncryptionMaterial(node *tree.Node) (encrypt.Materials, error) {

	encResp, encErr := e.getClient().GetEncryptionKey(context.Background(), &tree.GetEncryptionKeyRequest{
		Node:     node,
		Create:   true,
		User:     "bob",
		Password: "bobsecret",
	})
	if encErr != nil {
		return nil, encErr
	}
	symmetricKey := encrypt.NewSymmetricKey(encResp.Key)
	material, encErr2 := encrypt.NewCBCSecureMaterials(symmetricKey)
	if encErr2 != nil {
		return nil, encErr2
	}
	return material, nil

}

// Enrich request metadata for GetObject with Encryption Materials, if required by datasource
func (e *EncryptionHandler) GetObject(ctx context.Context, node *tree.Node, requestData *GetRequestData) (io.ReadCloser, error) {

	info, ok := GetBranchInfo(ctx, "in")
	if ok && info.Encrypted {

		material, err := e.retrieveEncryptionMaterial(node)
		if err == nil {
			requestData.EncryptionMaterial = material
		}

	}
	return e.next.GetObject(ctx, node, requestData)
}

// Enrich request metadata for PutObject with Encryption Materials, if required by datasource
func (e *EncryptionHandler) PutObject(ctx context.Context, node *tree.Node, reader io.Reader, requestData *PutRequestData) (int64, error) {

	info, ok := GetBranchInfo(ctx, "in")
	if ok && info.Encrypted {

		material, err := e.retrieveEncryptionMaterial(node)
		if err == nil {
			requestData.EncryptionMaterial = material
		}

	}

	return e.next.PutObject(ctx, node, reader, requestData)
}

// Enrich request metadata for CopyObject with Encryption Materials, if required by datasource
func (e *EncryptionHandler) CopyObject(ctx context.Context, from *tree.Node, to *tree.Node, requestData *CopyRequestData) (int64, error) {

	info, ok := GetBranchInfo(ctx, "to")
	if ok && info.Encrypted {

		material, err := e.retrieveEncryptionMaterial(to)
		if err == nil {
			requestData.destEncryptionMaterial = material
		}

	}
	srcInfo, ok := GetBranchInfo(ctx, "from")
	if ok && srcInfo.Encrypted {

		material, err := e.retrieveEncryptionMaterial(from)
		if err == nil {
			requestData.srcEncryptionMaterial = material
		}
	}

	return e.next.CopyObject(ctx, from, to, requestData)
}
