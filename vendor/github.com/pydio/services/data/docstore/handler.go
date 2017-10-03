package docstore

import (
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/docstore"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type Handler struct {
	Db      Store
	Indexer Indexer
}

func (h *Handler) Close() error {
	var err error
	err = h.Db.Close()
	err = h.Indexer.Close()
	return err
}

func (h *Handler) PutDocument(ctx context.Context, request *docstore.PutDocumentRequest, response *docstore.PutDocumentResponse) error {
	e := h.Db.PutDocument(request.StoreID, request.Document)
	if e != nil {
		log.Logger(ctx).Error("PutDocument", zap.Error(e))
		return e
	}
	e = h.Indexer.IndexDocument(request.StoreID, request.Document)
	if e != nil {
		log.Logger(ctx).Error("PutDocument:Index", zap.Error(e))
		return e
	}
	response.Document = request.Document
	return e
}

func (h *Handler) GetDocument(ctx context.Context, request *docstore.GetDocumentRequest, response *docstore.GetDocumentResponse) error {
	doc, e := h.Db.GetDocument(request.StoreID, request.DocumentID)
	if e != nil {
		return nil
	}
	response.Document = doc
	return nil
}

func (h *Handler) DeleteDocuments(ctx context.Context, request *docstore.DeleteDocumentsRequest, response *docstore.DeleteDocumentsResponse) error {

	if request.Query != nil && request.Query.MetaQuery != "" {

		docIds, err := h.Indexer.SearchDocuments(request.StoreID, request.Query)
		if err != nil {
			return err
		}
		log.Logger(ctx).Debug("SEARCH RESULTS", zap.Any("docs", docIds))
		for _, docId := range docIds {
			if e := h.Db.DeleteDocument(request.StoreID, docId); e == nil {
				response.DeletionCount++
			}

		}
		response.Success = true
		return nil

	} else {

		err := h.Db.DeleteDocument(request.StoreID, request.DocumentID)
		if err != nil {
			return err
		}
		response.Success = true
		response.DeletionCount = 1
		return h.Indexer.DeleteDocument(request.StoreID, request.DocumentID)

	}
}

func (h *Handler) ListDocuments(ctx context.Context, request *docstore.ListDocumentsRequest, stream docstore.DocStore_ListDocumentsStream) error {

	log.Logger(ctx).Debug("ListDocuments", zap.Any("req", request))

	defer stream.Close()

	if request.Query != nil && request.Query.MetaQuery != "" {

		docIds, err := h.Indexer.SearchDocuments(request.StoreID, request.Query)
		if err != nil {
			return err
		}
		for _, docId := range docIds {
			if doc, e := h.Db.GetDocument(request.StoreID, docId); e == nil && doc != nil {
				doc.ID = docId
				stream.Send(&docstore.ListDocumentsResponse{Document: doc})
			}
		}

	} else {

		results, done, err := h.Db.ListDocuments(request.StoreID, request.Query)

		if err != nil {
			return err
		}

		defer close(results)
		for {
			select {
			case doc := <-results:
				stream.Send(&docstore.ListDocumentsResponse{Document: doc})
			case <-done:
				return nil
			}
		}

	}

	return nil
}
