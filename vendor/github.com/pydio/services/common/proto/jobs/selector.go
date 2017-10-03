package jobs

import (
	"github.com/micro/go-micro/client"
	service "github.com/pydio/services/common/service/proto"
	"golang.org/x/net/context"
)

type InputSelector interface {
	Select(cl client.Client, ctx context.Context, objects chan interface{}, done chan bool) error
	MultipleSelection() bool
}

type InputFilter interface {
	Filter(input ActionMessage) ActionMessage
}

func reduceQueryBooleans(results []bool, operation service.OperationType) bool {

	reduced := true
	if operation == service.OperationType_AND {
		// If one is false, it's false
		for _, b := range results {
			reduced = reduced && b
		}
	} else {
		// At least one must be true
		reduced = false
		for _, b := range results {
			reduced = reduced || b
			if b {
				break
			}
		}
	}
	return reduced

}
