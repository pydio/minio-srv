package images

import "github.com/pydio/services/scheduler/actions"

// Auto register image-related tasks
func init() {

	manager := actions.GetActionsManager()

	manager.Register(thumbnailsActionName, func() actions.ConcreteAction {
		return &ThumbnailExtractor{}
	})

	manager.Register(exifTaskName, func() actions.ConcreteAction {
		return &ExifProcessor{}
	})

	manager.Register(cleanThumbTaskName, func() actions.ConcreteAction {
		return &CleanThumbsTask{}
	})

}
