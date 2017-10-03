package jobs

import (
	"strings"

	"github.com/micro/protobuf/ptypes"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/proto/tree"
	service "github.com/pydio/services/common/service/proto"
)

func (n *SourceFilter) Filter(input ActionMessage) ActionMessage {

	results := []bool{}

	for _, q := range n.GetQuery().GetSubQueries() {

		outputQuery := &service.ActionOutputQuery{}
		err := ptypes.UnmarshalAny(q, outputQuery)
		if err == nil && outputQuery != nil && input.GetLastOutput() != nil {
			pass := n.filterOutput(outputQuery, input.GetLastOutput())
			if outputQuery.Not {
				pass = !pass
			}
			results = append(results, pass)
		}

		sourceQuery := &service.SourceSingleQuery{}
		err = ptypes.UnmarshalAny(q, sourceQuery)
		if err == nil && sourceQuery != nil {
			pass := n.filterSource(sourceQuery, input)
			if sourceQuery.Not {
				pass = !pass
			}
			results = append(results, pass)
		}

	}

	if !reduceQueryBooleans(results, n.Query.Operation) {
		output := input
		// Filter out all future message actions
		output.Nodes = []*tree.Node{}
		output.Users = []*idm.User{}
		return output
	}

	return input
}

func (n *SourceFilter) filterOutput(query *service.ActionOutputQuery, output *ActionOutput) bool {

	if query.Success && !output.Success {
		return false
	}

	if query.Failed && output.Success {
		return false
	}

	if len(query.StringBodyCompare) > 0 && !strings.Contains(output.StringBody, query.StringBodyCompare) {
		return false
	}

	if len(query.ErrorStringCompare) > 0 && !strings.Contains(output.ErrorString, query.ErrorStringCompare) {
		return false
	}

	// TODO - HANDLE JSON

	return true
}

func (n *SourceFilter) filterSource(query *service.SourceSingleQuery, input ActionMessage) bool {
	return true
}
