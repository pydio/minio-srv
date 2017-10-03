package sql

import (
	"fmt"
	"strings"

	google_protobuf "github.com/golang/protobuf/ptypes/any"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/service/proto"

	"github.com/micro/protobuf/ptypes"
)

// Enquirer interface
type Enquirer interface {
	GetSubQueries() []*google_protobuf.Any
	GetOperation() service.OperationType
	GetOffset() int64
	GetLimit() int64
	GetGroupBy() int32

	fmt.Stringer
}

// Query redefinition for the DAO
type query struct {
	enquirer   Enquirer
	converters []common.Converter
}

// NewDAOQuery adds database functionality to a Query proto message
func NewDAOQuery(enquirer Enquirer, converters ...common.Converter) fmt.Stringer {
	return query{
		enquirer,
		converters,
	}
}

// Build a Query from a proto message and a list of converters
func (q query) String() string {

	var wheres []string
	var sqlString string

	for _, subQ := range q.enquirer.GetSubQueries() {

		sub := new(service.Query)

		if ptypes.Is(subQ, sub) {

			if err := ptypes.UnmarshalAny(subQ, sub); err != nil {
				// TODO something
			}

			subQueryString := NewDAOQuery(sub, q.converters...).String()

			wheres = append(wheres, subQueryString)
		} else {
			for _, converter := range q.converters {
				if str, ok := converter.Convert(subQ); ok {
					wheres = append(wheres, str)
				}
			}
		}
	}

	var join string
	if q.enquirer.GetOperation() == service.OperationType_AND {
		join = "AND"
	} else {
		join = "OR"
	}
	if len(wheres) > 1 {
		sqlString = "(" + strings.Join(wheres, ") "+join+" (") + ")"
	} else {
		sqlString = strings.Join(wheres, "")
	}

	return sqlString
}

// GetQueryValueFor field value
func GetQueryValueFor(field string, values ...string) string {

	if len(values) > 1 {
		var quoted []string

		for _, s := range values {
			quoted = append(quoted, "'"+s+"'")
		}

		return fmt.Sprintf("%v in (%v)", field, strings.Join(quoted, ","))

	} else if len(values) == 1 {
		if strings.Contains(values[0], "*") {
			return fmt.Sprintf("%s LIKE '%s'", field, strings.Replace(values[0], "*", "%", -1))
		}

		return fmt.Sprintf("%v='%v'", field, values[0])
	}

	return ""
}
