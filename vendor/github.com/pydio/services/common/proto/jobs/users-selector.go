package jobs

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common/proto/idm"
	"golang.org/x/net/context"
)

func (u *UsersSelector) MultipleSelection() bool{
	return u.Collect
}

// ENRICH UsersSelector METHODS
func (u *UsersSelector) Select(client client.Client, ctx context.Context, objects chan interface{}, done chan bool) error{
	objects <- &idm.User{
		ID:"admin",
	}
	objects <- &idm.User{
		ID:"user1",
	}
	objects <- &idm.User{
		ID:"user2",
	}
	done <- true
	return nil
}

func (n *UsersSelector) Filter(input ActionMessage) (ActionMessage) {
	return input
}
