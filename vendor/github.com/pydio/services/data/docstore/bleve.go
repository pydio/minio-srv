package docstore

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/blevesearch/bleve"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/docstore"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type BleveServer struct {
	// Internal Bleve database
	Engine bleve.Index
	// For Testing purpose : delete file after closing
	DeleteOnClose bool
	// Path to the DB file
	IndexPath string
}

func NewBleveEngine(bleveIndexPath string, deleteOnClose ...bool) (*BleveServer, error) {

	_, e := os.Stat(bleveIndexPath)
	var index bleve.Index
	var err error
	if e == nil {
		index, err = bleve.Open(bleveIndexPath)
	} else {
		index, err = bleve.New(bleveIndexPath, bleve.NewIndexMapping())
	}
	if err != nil {
		return nil, err
	}
	del := false
	if len(deleteOnClose) > 0 && deleteOnClose[0] {
		del = true
	}
	return &BleveServer{
		Engine:        index,
		IndexPath:     bleveIndexPath,
		DeleteOnClose: del,
	}, nil

}

func (s *BleveServer) Close() error {

	err := s.Engine.Close()
	if s.DeleteOnClose {
		err = os.RemoveAll(s.IndexPath)
	}
	return err

}

func (s *BleveServer) IndexDocument(storeID string, doc *docstore.Document) error {

	if doc.IndexableMeta == nil {
		return nil
	}
	toIndex := make(map[string]interface{})
	err := json.Unmarshal(doc.IndexableMeta, &toIndex)
	if err != nil {
		return nil
	}
	toIndex["DOCSTORE_STORE_ID"] = storeID
	toIndex["DOCSTORE_DOC_ID"] = doc.GetID()
	if doc.GetOwner() != "" {
		toIndex["DOCSTORE_OWNER"] = doc.GetOwner()
	}
	log.Logger(context.Background()).Info("IndexDocument", zap.Any("data", toIndex))
	err = s.Engine.Index(doc.GetID(), toIndex)
	if err != nil {
		return err
	}
	return nil
}

func (s *BleveServer) DeleteDocument(storeID string, docID string) error {

	return s.Engine.Delete(docID)

}

func (s *BleveServer) ClearIndex(ctx context.Context) error {
	// List all nodes and remove them
	request := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	MaxUint := ^uint(0)
	MaxInt := int(MaxUint >> 1)
	request.Size = MaxInt
	searchResult, err := s.Engine.Search(request)
	if err != nil {
		return err
	}
	for _, hit := range searchResult.Hits {
		log.Logger(ctx).Info("ClearIndex", zap.String("hit", hit.ID))
		s.Engine.Delete(hit.ID)
	}
	return nil
}

func (s *BleveServer) SearchDocuments(storeID string, query *docstore.DocumentQuery) ([]string, error) {

	parts := strings.Split(query.MetaQuery, " ")
	for i, p := range parts {
		if !strings.HasPrefix(p, "+") && !strings.HasPrefix(p, "-") {
			parts[i] = "+" + p
		}
	}

	parts = append(parts, " +DOCSTORE_STORE_ID:"+s.escapeMetaValue(storeID))
	if len(query.Owner) > 0 {
		parts = append(parts, " +DOCSTORE_OWNER:"+s.escapeMetaValue(query.Owner))
	}
	qStringQuery := bleve.NewQueryStringQuery(strings.Join(parts, " "))

	log.Logger(context.Background()).Info("SearchDocuments", zap.Any("query", qStringQuery))
	searchRequest := bleve.NewSearchRequest(qStringQuery)

	// TODO PASS CURSOR INFOS?
	searchRequest.Size = int(100)
	searchRequest.From = int(0)

	docs := []string{}
	searchResult, err := s.Engine.Search(searchRequest)
	if err != nil {
		return docs, err
	}
	log.Logger(context.Background()).Info("SearchDocuments", zap.Any("result", searchResult))
	for _, hit := range searchResult.Hits {
		doc, docErr := s.Engine.Document(hit.ID)
		if docErr != nil || doc == nil || doc.ID == "" {
			continue
		}
		docs = append(docs, doc.ID)
	}

	return docs, nil

}

func (s *BleveServer) escapeMetaValue(value string) string {

	r := strings.NewReplacer("-", "\\-", "~", "\\~", "*", "\\*", ":", "\\:", "/", "\\/", " ", "\\ ")
	return r.Replace(value)

}
