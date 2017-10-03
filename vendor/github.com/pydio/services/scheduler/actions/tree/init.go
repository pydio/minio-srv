package tree

import "github.com/pydio/services/scheduler/actions"

func init() {

	manager := actions.GetActionsManager()

	manager.Register(copyMoveActionName, func() actions.ConcreteAction {
		return &CopyMoveAction{}
	})

	manager.Register(deleteActionName, func() actions.ConcreteAction {
		return &DeleteAction{}
	})

	manager.Register(metaActionName, func() actions.ConcreteAction {
		return &MetaAction{}
	})

	manager.Register(snapshotActionName, func() actions.ConcreteAction {
		return &SnapshotAction{}
	})

}
