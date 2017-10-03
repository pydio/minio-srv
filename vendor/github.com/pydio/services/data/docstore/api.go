package docstore

import (
	"encoding/json"

	"github.com/micro/cli"
	"github.com/micro/go-micro"
	"github.com/micro/go-micro/errors"
	"github.com/micro/go-micro/server"
	api "github.com/micro/micro/api/proto"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/auth"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/docstore"
	"github.com/pydio/services/common/service"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type Docstore struct {
	DocStoreClient docstore.DocStoreClient
}

func (s *Docstore) Get(ctx context.Context, req *api.Request, rsp *api.Response) error {

	var storeId string
	var docId string
	if storeIdParams, ok := req.GetGet()["store_id"]; ok {
		storeId = storeIdParams.Values[0]
	}
	if docIdParams, ok := req.GetGet()["doc_id"]; ok {
		docId = docIdParams.Values[0]
	}
	if len(storeId) == 0 || len(docId) == 0 {
		return errors.BadRequest(common.SERVICE_DOCSTORE, "Missing StoreID or DocID")
	}

	notFound := false
	resp, err := s.DocStoreClient.GetDocument(ctx, &docstore.GetDocumentRequest{StoreID: storeId, DocumentID: docId})
	if err != nil {
		if errors.Parse(err.Error()).Code == 404 {
			notFound = true
		} else {
			log.Logger(ctx).Error("Error retrieving document", zap.Error(err))
			return err
		}
	}
	if resp.Document == nil || notFound {
		rsp.StatusCode = 404
		rsp.Body = "{}"
	} else {

		rsp.StatusCode = 200
		rsp.Header = make(map[string]*api.Pair, 1)
		rsp.Header["Content-type"] = &api.Pair{
			Key:    "Content-type",
			Values: []string{"application/json; charset=utf8"},
		}

		rsp.Body = string(resp.Document.Data)

	}

	return nil

}

func (s *Docstore) Delete(ctx context.Context, req *api.Request, rsp *api.Response) error {

	var storeId string
	var docId string
	if storeIdParams, ok := req.GetGet()["store_id"]; ok {
		storeId = storeIdParams.Values[0]
	}
	if len(storeId) == 0 {
		return errors.BadRequest(common.SERVICE_DOCSTORE, "Missing StoreID")
	}
	if docIdParams, ok := req.GetGet()["doc_id"]; ok {
		docId = docIdParams.Values[0]
	}
	query := &docstore.DocumentQuery{}
	if metaQParams, ok := req.GetGet()["meta_query"]; ok {
		query.MetaQuery = metaQParams.Values[0]
	}
	if ownerParams, ok := req.GetGet()["owner"]; ok {
		query.Owner = ownerParams.Values[0]
	}

	if len(docId) == 0 && len(query.MetaQuery) == 0 && len(query.Owner) == 0 {
		return errors.BadRequest(common.SERVICE_DOCSTORE, "Please provide at least one criteria for deletion")
	}

	deleteResponse, err := s.DocStoreClient.DeleteDocuments(ctx, &docstore.DeleteDocumentsRequest{
		StoreID:    storeId,
		DocumentID: docId,
		Query:      query,
	})

	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}

	if err != nil {
		if errors.Parse(err.Error()).Code == 404 {
			rsp.StatusCode = 404
			rsp.Body = `{"Success":false}`
		} else {
			log.Logger(ctx).Error("Error retrieving document", zap.Error(err))
			return err
		}
	} else {
		rsp.StatusCode = 200
		data, _ := json.Marshal(deleteResponse)
		rsp.Body = string(data)
	}

	return nil

}

func (s *Docstore) Put(ctx context.Context, req *api.Request, rsp *api.Response) error {

	var storeId string
	typeBinary := false
	if storeIdParams, ok := req.GetGet()["store_id"]; ok {
		storeId = storeIdParams.Values[0]
	}
	if len(storeId) == 0 {
		return errors.BadRequest(common.SERVICE_DOCSTORE, "Missing StoreID")
	}
	if typeParams, ok := req.GetGet()["type"]; ok {
		if typeParams.Values[0] == "binary" {
			typeBinary = true
		}
	}

	doc := &docstore.Document{
		Type: docstore.DocumentType_JSON,
	}

	body := make(map[string]interface{})
	err := json.Unmarshal([]byte(req.Body), &body)
	if err != nil {
		return errors.BadRequest(common.SERVICE_DOCSTORE, "Could not deserialize body:"+err.Error())
	}
	if id, ok := body["id"]; ok {
		doc.ID = id.(string)
	} else {
		return errors.BadRequest(common.SERVICE_DOCSTORE, "Please provide an Uuid for document")
	}
	if typeBinary {
		// Binary Type: Store URL
		doc.Type = docstore.DocumentType_BINARY
		s3Key := storeId + "-" + doc.ID
		docUrl := map[string]string{
			"URL": common.PYDIO_DOCSTORE_BINARIES_NAMESPACE + "/" + s3Key,
		}
		doc.Data, _ = json.Marshal(docUrl)

	} else if data, ok := body["data"]; ok {
		// Binary Type: Store Data
		doc.Data, _ = json.Marshal(data)
	}

	if index, ok := body["index"]; ok {
		doc.IndexableMeta, _ = json.Marshal(index)
	}
	if owner, ok := body["owner"]; ok {
		doc.Owner = owner.(string)
	}

	log.Logger(ctx).Debug("Put Document", zap.Any("doc", doc))
	resp, err := s.DocStoreClient.PutDocument(ctx, &docstore.PutDocumentRequest{
		StoreID:  storeId,
		Document: doc,
	})
	if err != nil {
		log.Logger(ctx).Error("Error storing actual document", zap.Error(err))
		return err
	}

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}
	rsp.Body = string(resp.Document.Data)

	return nil

}

func (s *Docstore) List(ctx context.Context, req *api.Request, rsp *api.Response) error {

	var storeId string
	query := &docstore.DocumentQuery{}
	if storeIdParams, ok := req.GetGet()["store_id"]; ok {
		storeId = storeIdParams.Values[0]
	}
	if metaQParams, ok := req.GetGet()["meta_query"]; ok {
		query.MetaQuery = metaQParams.Values[0]
	}
	if ownerParams, ok := req.GetGet()["owner"]; ok {
		query.Owner = ownerParams.Values[0]
	}
	if len(storeId) == 0 {
		return errors.BadRequest(common.SERVICE_DOCSTORE, "Missing StoreID")
	}

	log.Logger(ctx).Debug("ListDocuments", zap.Any("req", req))

	// List all Jobs w. tasks
	streamer, err := s.DocStoreClient.ListDocuments(ctx, &docstore.ListDocumentsRequest{
		StoreID: storeId,
		Query:   query,
	})
	if err != nil {
		return err
	}
	results := make(map[string]interface{})

	for {
		resp, err := streamer.Recv()
		if err != nil {
			break
		}
		if resp == nil {
			continue
		}
		var unmarshalled interface{}
		json.Unmarshal(resp.Document.Data, &unmarshalled)
		results[resp.Document.ID] = unmarshalled
	}

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}

	encoded, e := json.Marshal(results)
	if e != nil {
		return e
	}
	rsp.Body = string(encoded)
	log.Logger(ctx).Debug("LIST OUTPUT", zap.String("out", rsp.Body))

	return nil

}

func apiBuilder(service micro.Service) interface{} {
	return &Docstore{
		DocStoreClient: docstore.NewDocStoreClient(common.SERVICE_DOCSTORE, service.Client()),
	}
}

// Starts the API
// Then Start micro --client=grpc api --namespace="pydio.service.api"
// Then call e.g. http://localhost:8080/jobs/list"
func NewDocStoreApiService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(apiBuilder,
		micro.Name(common.SERVICE_API_NAMESPACE_+"docstore"),
		micro.WrapHandler(newStoreIDChecker()),
	)

	/*service := templates.NewAnonApiService("docstore", apiBuilder, micro.WrapHandler(NewStoreIDChecker()))
	return service, nil*/

	return srv, nil
}

func newStoreIDChecker() server.HandlerWrapper {

	return func(h server.HandlerFunc) server.HandlerFunc {

		return func(ctx context.Context, req server.Request, rsp interface{}) error {

			if claims := ctx.Value(auth.PYDIO_CONTEXT_CLAIMS_KEY); claims != nil {
				// There are claims, do nothing
				return h(ctx, req, rsp)
			}

			if httpRequest, ok := (req.Request()).(*api.Request); ok {
				if s, sOk := httpRequest.GetGet()["store_id"]; sOk {
					storeId := s.Values[0]
					if storeId == "password-reset" {
						return h(ctx, req, rsp)
					}
				}
			}

			return errors.Forbidden(common.SERVICE_DOCSTORE, "Forbidden store access")

		}
	}
}
