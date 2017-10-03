package versions

import "github.com/pydio/services/scheduler/actions"

func init() {

	manager := actions.GetActionsManager()

	manager.Register(versionActionName, func() actions.ConcreteAction {
		return &VersionAction{}
	})
}