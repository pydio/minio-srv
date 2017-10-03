package cmd

import "github.com/pydio/services/scheduler/actions"

func init(){

	manager := actions.GetActionsManager()

	manager.Register(rpcActionName, func() actions.ConcreteAction {
		return &RpcAction{}
	})

	manager.Register(shellActionName, func() actions.ConcreteAction {
		return &ShellAction{}
	})

	manager.Register(wgetActionName, func() actions.ConcreteAction {
		return &WGetAction{}
	})


}