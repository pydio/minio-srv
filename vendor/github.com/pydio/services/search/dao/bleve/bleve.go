package bleve

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/blevesearch/bleve"
	_ "github.com/blevesearch/bleve/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/search/query"
	"github.com/pydio/services/common/config"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"github.com/sajari/docconv"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

var (
	BleveIndexPath = ""
)

type BleveServer struct {
	Engine       bleve.Index
	IndexContent bool
}

func NewBleveEngine(indexContent bool) (*BleveServer, error) {

	if BleveIndexPath == "" {
		filePath, err := config.ApplicationDataDir()
		if err != nil {
			return nil, err
		}
		BleveIndexPath = path.Join(filePath, "searchengine.bleve")
	}

	_, e := os.Stat(BleveIndexPath)
	var index bleve.Index
	var err error
	if e == nil {

		index, err = bleve.Open(BleveIndexPath)

	} else {

		mapping := bleve.NewIndexMapping()
		nodeMapping := bleve.NewDocumentMapping()
		mapping.AddDocumentMapping("node", nodeMapping)

		// Path to keyword
		pathFieldMapping := bleve.NewTextFieldMapping()
		pathFieldMapping.Analyzer = "keyword"
		nodeMapping.AddFieldMappingsAt("Path", pathFieldMapping)

		// Node type to keyword
		nodeType := bleve.NewTextFieldMapping()
		nodeType.Analyzer = "keyword"
		nodeMapping.AddFieldMappingsAt("NodeType", nodeType)

		// Extension to keyword
		extType := bleve.NewTextFieldMapping()
		extType.Analyzer = "keyword"
		nodeMapping.AddFieldMappingsAt("Extension", extType)

		// Modification Time as Date
		modifTime := bleve.NewDateTimeFieldMapping()
		nodeMapping.AddFieldMappingsAt("ModifTime", modifTime)

		// GeoPoint
		geoPosition := bleve.NewGeoPointFieldMapping()
		nodeMapping.AddFieldMappingsAt("GeoPoint", geoPosition)

		// Text Content
		textContent := bleve.NewTextFieldMapping()
		textContent.Analyzer = "en" // See detect_lang in the blevesearch/blevex package?
		textContent.Store = false
		textContent.IncludeInAll = false
		nodeMapping.AddFieldMappingsAt("TextContent", textContent)

		index, err = bleve.New(BleveIndexPath, mapping)
	}
	if err != nil {
		return nil, err
	}
	return &BleveServer{
		Engine:       index,
		IndexContent: indexContent,
	}, nil

}

type IndexableNode struct {
	tree.Node
	ModifTime   time.Time
	Basename    string
	NodeType    string
	Extension   string
	TextContent string
	GeoPoint    map[string]interface{}
	Meta        map[string]interface{}
}

func (i *IndexableNode) BleveType() string {
	return "node"
}

func (s *BleveServer) MakeIndexableNode(ctx context.Context, node *tree.Node) *IndexableNode {
	indexNode := &IndexableNode{Node: *node}
	indexNode.Meta = indexNode.AllMetaDeserialized()
	indexNode.ModifTime = time.Unix(indexNode.MTime, 0)
	var basename string
	indexNode.GetMeta("name", &basename)
	indexNode.Basename = basename
	if indexNode.Type == 1 {
		indexNode.NodeType = "file"
		indexNode.Extension = filepath.Ext(basename)
	} else {
		indexNode.NodeType = "folder"
	}
	indexNode.GetMeta("GeoLocation", &indexNode.GeoPoint)

	if s.IndexContent && indexNode.IsLeaf() {
		reader, err := indexNode.ReadFile(ctx)
		if err == nil {
			convertResp, er := docconv.Convert(reader, docconv.MimeTypeByExtension(basename), true)
			if er == nil {
				// Todo : do something with convertResp.Meta?
				log.Logger(ctx).Debug("[BLEVE] Indexing content body for file")
				indexNode.TextContent = convertResp.Body
			}
		} else {
			log.Logger(ctx).Debug("[BLEVE] Index content: error while trying to read file for content indexation")
		}
	}
	indexNode.MetaStore = nil
	return indexNode
}

func (s *BleveServer) Close() error {

	return s.Engine.Close()

}

func (s *BleveServer) IndexNode(c context.Context, n *tree.Node) error {

	indexNode := s.MakeIndexableNode(c, n)
	err := s.Engine.Index(n.GetUuid(), indexNode)

	log.Logger(c).Info("IndexNode", zap.Any("node", indexNode))

	if err != nil {
		return err
	}
	return nil
}

func (s *BleveServer) DeleteNode(c context.Context, n *tree.Node) error {

	return s.Engine.Delete(n.GetUuid())

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

func (s *BleveServer) SearchNodes(c context.Context, queryObject *tree.Query, from int32, size int32, resultChan chan *tree.Node, doneChan chan bool) error {

	boolean := bleve.NewBooleanQuery()
	// FileName
	if len(queryObject.GetFileName()) > 0 {
		wCard := bleve.NewWildcardQuery("*" + strings.Trim(strings.ToLower(queryObject.GetFileName()), "*") + "*")
		wCard.SetField("Basename")
		boolean.AddMust(wCard)
	}
	// File Size Range
	if queryObject.MinSize > 0 || queryObject.MaxSize > 0 {
		var min = float64(queryObject.MinSize)
		var max = float64(queryObject.MaxSize)
		var numRange *query.NumericRangeQuery
		if max == 0 {
			numRange = bleve.NewNumericRangeQuery(&min, nil)
		} else {
			numRange = bleve.NewNumericRangeQuery(&min, &max)
		}
		numRange.SetField("Size")
		boolean.AddMust(numRange)
	}
	// Date Range
	if queryObject.MinDate > 0 || queryObject.MaxDate > 0 {
		var dateRange *query.DateRangeQuery
		if queryObject.MaxDate > 0 {
			dateRange = bleve.NewDateRangeQuery(time.Unix(queryObject.MinDate, 0), time.Unix(queryObject.MaxDate, 0))
		} else {
			dateRange = bleve.NewDateRangeQuery(time.Unix(queryObject.MinDate, 0), time.Now())
		}
		dateRange.SetField("ModifTime")
		boolean.AddMust(dateRange)
	}
	// Limit to a SubTree
	if len(queryObject.PathPrefix) > 0 {
		subQ := bleve.NewBooleanQuery()
		for _, pref := range queryObject.PathPrefix {
			prefix := bleve.NewPrefixQuery(pref)
			prefix.SetField("Path")
			subQ.AddShould(prefix)
		}
		boolean.AddMust(subQ)
	}
	// Limit to a given node type
	if queryObject.Type > 0 {
		nodeType := "file"
		if queryObject.Type == 2 {
			nodeType = "folder"
		}
		typeQuery := bleve.NewTermQuery(nodeType)
		typeQuery.SetField("NodeType")
		boolean.AddMust(typeQuery)
	}

	if len(queryObject.Extension) > 0 {
		extQuery := bleve.NewTermQuery(queryObject.Extension)
		extQuery.SetField("Extension")
		boolean.AddMust(extQuery)
	}

	if len(queryObject.FreeString) > 0 {
		qStringQuery := bleve.NewQueryStringQuery(queryObject.FreeString)
		boolean.AddMust(qStringQuery)
	}

	if queryObject.GeoQuery != nil {
		if queryObject.GeoQuery.Center != nil && len(queryObject.GeoQuery.Distance) > 0 {
			distanceQuery := bleve.NewGeoDistanceQuery(queryObject.GeoQuery.Center.Lon, queryObject.GeoQuery.Center.Lat, queryObject.GeoQuery.Distance)
			distanceQuery.SetField("GeoPoint")
			boolean.AddMust(distanceQuery)
		} else if queryObject.GeoQuery.TopLeft != nil && queryObject.GeoQuery.BottomRight != nil {
			boundingBoxQuery := bleve.NewGeoBoundingBoxQuery(
				queryObject.GeoQuery.TopLeft.Lon,
				queryObject.GeoQuery.TopLeft.Lat,
				queryObject.GeoQuery.BottomRight.Lon,
				queryObject.GeoQuery.BottomRight.Lat,
			)
			boundingBoxQuery.SetField("GeoPoint")
			boolean.AddMust(boundingBoxQuery)
		}
	}

	log.Logger(c).Info("SearchObjects", zap.Any("query", boolean))
	searchRequest := bleve.NewSearchRequest(boolean)
	if size > 0 {
		searchRequest.Size = int(size)
	}
	searchRequest.From = int(from)
	searchResult, err := s.Engine.SearchInContext(c, searchRequest)
	if err != nil {
		doneChan <- true
		return err
	}
	log.Logger(c).Info("SearchObjects", zap.Any("result", searchResult))
	for _, hit := range searchResult.Hits {
		doc, docErr := s.Engine.Document(hit.ID)
		if docErr != nil || doc == nil {
			continue
		}
		node := &tree.Node{}
		for _, f := range doc.Fields {
			stringValue := string(f.Value())
			switch f.Name() {
			case "Uuid":
				node.Uuid = stringValue
				break
			case "Path":
				node.Path = stringValue
				break
			case "NodeType":
				if stringValue == "file" {
					node.Type = 1
				} else if stringValue == "folder" {
					node.Type = 2
				}
				break
			case "Basename":
				node.SetMeta("name", stringValue)
			default:
				break
			}
		}

		log.Logger(c).Info("SearchObjects", zap.Any("node", node))

		resultChan <- node
	}

	doneChan <- true
	return nil

}
