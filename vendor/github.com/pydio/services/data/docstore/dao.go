package docstore

import (
	"github.com/pydio/services/common/proto/docstore"
)

type Store interface{
	PutDocument(storeID string, doc *docstore.Document) error
	GetDocument(storeID string, docId string) (*docstore.Document, error)
	DeleteDocument(storeID string, docID string) error
	ListDocuments(storeID string, query *docstore.DocumentQuery) (chan *docstore.Document, chan bool, error)
	Close() error
}

type Indexer interface{
	IndexDocument(storeID string, doc *docstore.Document) error
	DeleteDocument(storeID string, docID string) error
	SearchDocuments(storeID string, query *docstore.DocumentQuery) ([]string, error)
	Close() error
}
