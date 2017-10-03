package tree

import (
	"github.com/pborman/uuid"
)

func NewChangeLogFromNode(node *Node, description ...string) *ChangeLog{

	c := &ChangeLog{}
	c.Uuid = uuid.NewUUID().String()
	c.Data = []byte(node.Etag)
	c.MTime = node.MTime
	c.Size = node.Size
	if len(description) > 0 {
		c.Description = description[0]
	}

	return c

}