package jobs

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"sync"
)

func (a *Action) ToMessages(startMessage ActionMessage, c client.Client, ctx context.Context, output chan ActionMessage) {

	startMessage = a.ApplyFilters(startMessage)
	if a.HasSelectors() {
		a.ResolveSelectors(startMessage, c, ctx, output)
	} else {
		output <- startMessage
	}
}

func (a *Action) HasSelectors() bool {
	return len(a.getSelectors()) > 0
}

func (a *Action) getSelectors() []InputSelector {
	selectors := []InputSelector{}
	if a.NodesSelector != nil {
		selectors = append(selectors, a.NodesSelector)
	}
	if a.UsersSelector != nil {
		selectors = append(selectors, a.UsersSelector)
	}
	return selectors
}

func (a *Action) ApplyFilters(input ActionMessage) ActionMessage {
	if a.NodesFilter != nil {
		input = a.NodesFilter.Filter(input)
	}
	if a.UsersFilter != nil {
		input = a.UsersFilter.Filter(input)
	}
	if a.SourceFilter != nil {
		input = a.SourceFilter.Filter(input)
	}
	return input
}

func (a *Action) ResolveSelectors(startMessage ActionMessage, cl client.Client, ctx context.Context, output chan ActionMessage) {

	done := make(chan bool, 1)
	a.FanToNext(cl, ctx, 0, startMessage, output, done)

}

func (a *Action) FanToNext(cl client.Client, ctx context.Context, index int, input ActionMessage, output chan ActionMessage, done chan bool) {

	selectors := a.getSelectors()
	if index < len(selectors)-1 {
		// Make a intermediary pipes for output/done
		nextOut := make(chan ActionMessage)
		nextDone := make(chan bool, 1)
		go func() {
			for {
				select {
				case message := <-nextOut:
					go a.FanToNext(cl, ctx, index+1, message, output, done)
				case <-nextDone:
					close(nextOut)
					close(nextDone)
					return
				}
			}
		}()
		if selectors[index].MultipleSelection() {
			go a.CollectSelector(cl, ctx, selectors[index], input, nextOut, nextDone)
		} else {
			go a.FanOutSelector(cl, ctx, selectors[index], input, nextOut, nextDone)
		}
	} else {
		if selectors[index].MultipleSelection() {
			a.CollectSelector(cl, ctx, selectors[index], input, output, done)
		} else {
			a.FanOutSelector(cl, ctx, selectors[index], input, output, done)
		}
	}

}

func (a *Action) FanOutSelector(cl client.Client, ctx context.Context, selector InputSelector, input ActionMessage, output chan ActionMessage, done chan bool) {

	// If multiple selectors, we have to apply them sequentially
	outputType := ""
	if _, ok := selector.(*NodesSelector); ok {
		outputType = "node"
	} else if _, ok := selector.(*UsersSelector); ok {
		outputType = "user"
	}
	wire := make(chan interface{})
	selectDone := make(chan bool, 1)
	go func() {
		for {
			select {
			case object := <-wire:
				if outputType == "node" {
					nodeP := object.(*tree.Node)
					node := tree.Node(*nodeP)
					input = input.WithNode(&node)
					output <- input
				} else if outputType == "user" {
					userP := object.(*idm.User)
					user := idm.User(*userP)
					input = input.WithUser(&user)
					output <- input
				}
			case <-selectDone:
				close(wire)
				close(selectDone)
				done <- true
				return
			}
		}
	}()
	go selector.Select(cl, ctx, wire, selectDone)

}

func (a *Action) CollectSelector(cl client.Client, ctx context.Context, selector InputSelector, input ActionMessage, output chan ActionMessage, done chan bool) {

	// If multiple selectors, we have to apply them sequentially
	outputType := ""
	nodes := []*tree.Node{}
	users := []*idm.User{}
	if _, ok := selector.(*NodesSelector); ok {
		outputType = "node"
	} else if _, ok := selector.(*UsersSelector); ok {
		outputType = "user"
	}
	wire := make(chan interface{})
	selectDone := make(chan bool, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case object := <-wire:
				if outputType == "node" {
					nodeP := object.(*tree.Node)
					nodes = append(nodes, nodeP)
				} else if outputType == "user" {
					userP := object.(*idm.User)
					users = append(users, userP)
				}
			case <-selectDone:
				close(wire)
				close(selectDone)
				done <- true
				return
			}
		}
	}()
	go selector.Select(cl, ctx, wire, selectDone)
	wg.Wait()

	if outputType == "node" {
		input = input.WithNodes(nodes...)
	} else if outputType == "user" {
		input = input.WithUsers(users...)
	}
	output <- input

}
