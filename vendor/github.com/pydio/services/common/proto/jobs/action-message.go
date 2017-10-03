package jobs

import (
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/proto/idm"
)

func(a *ActionMessage) AppendOutput(output *ActionOutput){

	if a.OutputChain == nil {
		a.OutputChain = []*ActionOutput{}
	}

	a.OutputChain = append(a.OutputChain, output)

}

func(a *ActionMessage) GetLastOutput() *ActionOutput {

	if a.OutputChain == nil {
		return nil
	}

	return a.OutputChain[len(a.OutputChain)-1]

}

func (a *ActionMessage) GetOutputs() []*ActionOutput {

	if a.OutputChain == nil {
		a.OutputChain = []*ActionOutput{}
	}

	return a.OutputChain

}

func(a *ActionMessage) WithNode(n *tree.Node) ActionMessage {

	b := *a
	if n == nil {
		b.Nodes = []*tree.Node{}
	} else {
		b.Nodes = []*tree.Node{n}
	}
	return b

}

func(a *ActionMessage) WithNodes(nodes ...*tree.Node) ActionMessage {

	b := *a
	b.Nodes = []*tree.Node{}
	for _, n := range nodes{
		b.Nodes = append(b.Nodes, n)
	}
	return b

}

func(a *ActionMessage) WithUser(u *idm.User) ActionMessage {

	b := *a
	if u == nil {
		b.Users = []*idm.User{}
	} else {
		b.Users = []*idm.User{u}
	}
	return b

}

func(a *ActionMessage) WithUsers(users ...*idm.User) ActionMessage {

	b := *a
	b.Users = []*idm.User{}
	for _, u := range users{
		b.Users = append(b.Users, u)
	}
	return b

}

func(a *ActionMessage) WithError(e error) ActionMessage {

	b := *a
	b.AppendOutput(&ActionOutput{
		Success:false,
		ErrorString:e.Error(),
	})
	return b

}

func(a *ActionMessage) WithIgnore() ActionMessage {

	b := *a
	b.AppendOutput(&ActionOutput{
		Ignored:true,
	})
	return b

}