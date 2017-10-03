package service

import (
	"encoding/json"
	"errors"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/micro/protobuf/jsonpb"
	"github.com/micro/protobuf/proto"
	"github.com/micro/protobuf/ptypes"
	"github.com/mitchellh/mapstructure"
)

type ConcreteQuery struct {
	Name string      `json:"Name,omitempty"`
	Data interface{} `json:"Data,omitempty"`
}

type outputFormat struct {
	SubQueries []interface{} `json:"SubQueries,omitempty"`
	Operation  OperationType `json:"Operation,omitempty"`
	Offset     int64         `json:"Offset,omitempty"`
	Limit      int64         `json:"Limit,omitempty"`
	GroupBy    int32         `json:"groupBy,omitempty"`
}

func (q *Query) MarshalJSONPB(marshaler *jsonpb.Marshaler) ([]byte, error) {

	data := outputFormat{
		SubQueries: []interface{}{},
		Operation:  q.Operation,
		Offset:     q.Offset,
		Limit:      q.Limit,
		GroupBy:    q.GroupBy,
	}

	for _, obj := range q.SubQueries {
		if concrete, err := q.marshalAnyTypes(obj); err == nil {
			data.SubQueries = append(data.SubQueries, concrete)
		} else {
			data.SubQueries = append(data.SubQueries, obj)
		}
	}
	return json.Marshal(data)
}

func (q *Query) marshalAnyTypes(obj *any.Any) (*ConcreteQuery, error) {

	/*
		TODO - find a solution to register a marshaller
		nodeQ := &tree.Query{}
		if e := ptypes.UnmarshalAny(obj, nodeQ); e == nil {
			return &ConcreteQuery{Name: "tree.Query", Data: nodeQ}, nil
		}
	*/

	aoQ := &ActionOutputQuery{}
	if e := ptypes.UnmarshalAny(obj, aoQ); e == nil {
		return &ConcreteQuery{Name: "ActionOutputQuery", Data: aoQ}, nil
	}

	sourceQ := &SourceSingleQuery{}
	if e := ptypes.UnmarshalAny(obj, sourceQ); e == nil {
		return &ConcreteQuery{Name: "SourceSingleQuery", Data: sourceQ}, nil
	}

	return nil, errors.New("Type Not Found")
}

func (q *Query) UnmarshalJSONPB(unmarshaller *jsonpb.Unmarshaler, data []byte) error {

	input := &outputFormat{SubQueries: []interface{}{}}
	json.Unmarshal(data, input)
	q.GroupBy = input.GroupBy
	q.Limit = input.Limit
	q.Offset = input.Offset
	q.Operation = input.Operation
	q.SubQueries = []*any.Any{}
	for _, obj := range input.SubQueries {
		var casted ConcreteQuery
		if err := mapstructure.Decode(obj, &casted); err == nil {
			var o proto.Message
			switch casted.Name {
			/*case "tree.Query":
			o = &tree.Query{}
			*/
			case "ActionOutputQuery":
				o = &ActionOutputQuery{}
			case "SourceSingleQuery":
				o = &SourceSingleQuery{}
			}
			if e := mapstructure.Decode(casted.Data, o); e == nil {
				anyfied, _ := ptypes.MarshalAny(o)
				q.SubQueries = append(q.SubQueries, anyfied)
			}
		} else {
			var a any.Any
			if e := mapstructure.Decode(obj, &a); e == nil {
				q.SubQueries = append(q.SubQueries, &a)
			}

		}

	}

	return nil
}
