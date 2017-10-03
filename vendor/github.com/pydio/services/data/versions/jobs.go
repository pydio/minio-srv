package versions

import (
	"github.com/pydio/services/common/proto/jobs"
	"github.com/micro/protobuf/ptypes"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service/proto"
	"github.com/golang/protobuf/ptypes/any"
)

func getDefaultJobs() []*jobs.Job {

	searchQuery, _ := ptypes.MarshalAny(&tree.Query{
		Type: tree.NodeType_LEAF,
	})

	thumbnailsJob := &jobs.Job{
		ID:             "versionning-job",
		Owner:          "admin",
		Label:          "Event Based Job for replicating data for versioning",
		HasProgress:    false,
		Stoppable:      false,
		Inactive:       false,
		MaxConcurrency: 10,
		EventNames:     []string{"NODE_CHANGE:0", "NODE_CHANGE:3"}, // 0 = NodeChangeEvent_CREATE, 3 = NodeChangeEvent_UPDATE_CONTENT
		Actions: []*jobs.Action{
			{
				ID:         "actions.versioning.create",
				NodesFilter: &jobs.NodesSelector{
					Query: &service.Query{
						SubQueries: []*any.Any{searchQuery},
					},
				},
			},
		},
	}

/*
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

*/
	defJobs := []*jobs.Job{
		thumbnailsJob,
//		cleanThumbsJob,
	}

	return defJobs

}
