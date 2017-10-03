package jobs

import (
	"github.com/golang/protobuf/ptypes/any"
	"github.com/micro/protobuf/ptypes"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/proto/tree"
	service "github.com/pydio/services/common/service/proto"
)

func getDefaultJobs() []*jobs.Job {

	searchQuery, _ := ptypes.MarshalAny(&tree.Query{
		Extension: "jpg,png,jpeg,gif,bmp,tiff",
	})

	searchQueryExif, _ := ptypes.MarshalAny(&tree.Query{
		Extension: "jpg",
	})

	thumbnailsJob := &jobs.Job{
		ID:             "thumbs-job",
		Owner:          "admin",
		Label:          "Event Based Job for extracting image thumbnails",
		HasProgress:    false,
		Stoppable:      false,
		Inactive:       false,
		MaxConcurrency: 5,
		EventNames:     []string{"NODE_CHANGE:0", "NODE_CHANGE:3"}, // 0 = NodeChangeEvent_CREATE, 3 = NodeChangeEvent_UPDATE_CONTENT
		Actions: []*jobs.Action{
			{
				ID:         "actions.images.thumbnails",
				Parameters: map[string]string{"ThumbSizes": "256,512"},
				NodesFilter: &jobs.NodesSelector{
					Query: &service.Query{
						SubQueries: []*any.Any{searchQuery},
					},
				},
			},
			{
				ID: "actions.images.exif",
				NodesFilter: &jobs.NodesSelector{
					Query: &service.Query{
						SubQueries: []*any.Any{searchQueryExif},
					},
				},
			},
		},
	}

	cleanThumbsJob := &jobs.Job{
		ID:             "clean-thumbs-job",
		Owner:          "admin",
		Label:          "Event Based Job for cleaning thumbnails on node deletion",
		HasProgress:    false,
		Stoppable:      false,
		Inactive:       false,
		MaxConcurrency: 5,
		EventNames:     []string{"NODE_CHANGE:5"}, // 5 = NodeChangeEvent_DELETE
		Actions: []*jobs.Action{
			{
				ID: "actions.images.clean",
				NodesFilter: &jobs.NodesSelector{
					Query: &service.Query{
						SubQueries: []*any.Any{searchQuery},
					},
				},
			},
		},
	}

	defJobs := []*jobs.Job{
		thumbnailsJob,
		cleanThumbsJob,
	}

	return defJobs

}
