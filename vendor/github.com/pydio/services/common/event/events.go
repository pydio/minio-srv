package event

import (
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

// EventWithContext structure
type EventWithContext struct {
	Event   *tree.NodeChangeEvent
	Context context.Context
}
