package jobs

import (
	"encoding/json"

	"github.com/micro/cli"
	"github.com/micro/go-micro"
	api "github.com/micro/micro/api/proto"
	"github.com/micro/protobuf/jsonpb"
	"github.com/pborman/uuid"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/auth"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/common/views"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"strconv"
	"path/filepath"
)

type Jobs struct {
	JobsClient jobs.JobServiceClient
	Router     *views.Router
}

func (s *Jobs) Compress(ctx context.Context, req *api.Request, rsp *api.Response) error {

	var selectedPathes []string
	var targetNodePath string
	var format string

	log.Logger(ctx).Debug("Request POST Data", zap.Any("post", req.GetPost()))

	if pathes, ok := req.GetPost()["nodes[0]"]; ok {
		selectedPathes = append(selectedPathes, pathes.Values[0])
	}
	if targets, ok := req.GetPost()["archiveName"]; ok {
		targetNodePath = targets.Values[0]
	}
	if formats, ok := req.GetPost()["format"]; ok {
		format = formats.Values[0]
	} else {
		format = "zip"
	}

	jobUuid := uuid.NewUUID().String()
	claims := ctx.Value(auth.PYDIO_CONTEXT_CLAIMS_KEY).(auth.Claims)
	userName := claims.Name

	err := s.Router.WrapCallback(func(inputFilter views.NodeFilter, outputFilter views.NodeFilter) error {

		for i, path := range selectedPathes {
			node := &tree.Node{Path: path}
			_, nodeErr := inputFilter(ctx, node, "sel")
			log.Logger(ctx).Debug("Filtering Input Node", zap.Any("node", node), zap.Error(nodeErr))
			if nodeErr != nil {
				return nodeErr
			}
			selectedPathes[i] = node.Path
		}

		if targetNodePath != "" {
			node := &tree.Node{Path: targetNodePath}
			_, nodeErr := inputFilter(ctx, node, "sel")
			if nodeErr != nil {
				log.Logger(ctx).Error("Filtering Input Node", zap.Any("node", node), zap.Error(nodeErr))
				return nodeErr
			}
			targetNodePath = node.Path
		}

		log.Logger(ctx).Debug("Submitting selected pathes for compression", zap.Any("pathes", selectedPathes))

		job := &jobs.Job{
			ID:             "compress-folders-" + jobUuid,
			Owner:          userName,
			Label:          "Compressing selection",
			HasProgress:    false,
			Stoppable:      true,
			Inactive:       false,
			MaxConcurrency: 1,
			AutoStart:      true,
			Actions: []*jobs.Action{
				{
					ID: "actions.archive.compress",
					Parameters: map[string]string{
						"format": format,
						"target": targetNodePath, // NOT USED YES - TODO
					},
					NodesSelector: &jobs.NodesSelector{
						Collect: true,
						Pathes:  selectedPathes,
					},
				},
			},
		}

		_, er := s.JobsClient.PutJob(ctx, &jobs.PutJobRequest{Job: job})
		return er

	})

	result := make(map[string]bool, 1)
	if err == nil {
		result["success"] = true
	} else {
		result["success"] = false
	}
	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}
	encoded, e := json.Marshal(result)
	if e != nil {
		return e
	}
	rsp.Body = string(encoded)
	return nil

}

func (s *Jobs) Extract(ctx context.Context, req *api.Request, rsp *api.Response) error {

	var selectedNode string
	var targetPath string
	var format string

	if pathes, ok := req.GetPost()["node"]; ok {
		selectedNode = pathes.Values[0]
	}
	if targets, ok := req.GetPost()["target"]; ok {
		targetPath = targets.Values[0]
	}
	if formats, ok := req.GetPost()["format"]; ok {
		format = formats.Values[0]
	} else {
		format = "zip"
	}

	jobUuid := uuid.NewUUID().String()
	claims := ctx.Value(auth.PYDIO_CONTEXT_CLAIMS_KEY).(auth.Claims)
	userName := claims.Name

	err := s.Router.WrapCallback(func(inputFilter views.NodeFilter, outputFilter views.NodeFilter) error {

		node := &tree.Node{Path: selectedNode}
		_, nodeErr := inputFilter(ctx, node, "sel")
		if nodeErr != nil {
			log.Logger(ctx).Error("Filtering Input Node", zap.Any("node", node), zap.Error(nodeErr))
			return nodeErr
		}
		selectedNode = node.Path

		if targetPath != "" {
			node := &tree.Node{Path: targetPath}
			_, nodeErr := inputFilter(ctx, node, "sel")
			if nodeErr != nil {
				log.Logger(ctx).Error("Filtering Input Node", zap.Any("node", node), zap.Error(nodeErr))
				return nodeErr
			}
			targetPath = node.Path
		}

		job := &jobs.Job{
			ID:             "extract-archive-" + jobUuid,
			Owner:          userName,
			Label:          "Compressing selection",
			HasProgress:    false,
			Stoppable:      true,
			Inactive:       false,
			MaxConcurrency: 1,
			AutoStart:      true,
			Actions: []*jobs.Action{
				{
					ID: "actions.archive.extract",
					Parameters: map[string]string{
						"format": format,
						"target": targetPath,
					},
					NodesSelector: &jobs.NodesSelector{
						Pathes: []string{selectedNode},
					},
				},
			},
		}

		_, err := s.JobsClient.PutJob(ctx, &jobs.PutJobRequest{Job: job})
		return err

	})

	result := make(map[string]bool, 1)
	if err == nil {
		result["success"] = true
	} else {
		result["success"] = false
	}
	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}
	encoded, e := json.Marshal(result)
	if e != nil {
		return e
	}
	rsp.Body = string(encoded)
	return nil

}

func (s *Jobs) Dircopy(ctx context.Context, req *api.Request, rsp *api.Response) error {

	var selectedPathes []string
	var targetNodePath string
	taskType := "copy"

	log.Logger(ctx).Debug("Request POST Data", zap.Any("post", req.GetPost()))

	if pathes, ok := req.GetPost()["nodes[0]"]; ok {
		selectedPathes = append(selectedPathes, pathes.Values[0])
	}
	if targets, ok := req.GetPost()["target"]; ok {
		targetNodePath = targets.Values[0]
	}
	if moveParams, ok := req.GetPost()["move"]; ok {
		move, _ := strconv.ParseBool(moveParams.Values[0])
		if move {
			taskType = "move"
		}
	}

	jobUuid := uuid.NewUUID().String()
	claims := ctx.Value(auth.PYDIO_CONTEXT_CLAIMS_KEY).(auth.Claims)
	userName := claims.Name

	err := s.Router.WrapCallback(func(inputFilter views.NodeFilter, outputFilter views.NodeFilter) error {

		for i, path := range selectedPathes {
			node := &tree.Node{Path: path}
			_, nodeErr := inputFilter(ctx, node, "sel")
			log.Logger(ctx).Debug("Filtering Input Node", zap.Any("node", node), zap.Error(nodeErr))
			if nodeErr != nil {
				return nodeErr
			}
			selectedPathes[i] = node.Path
		}

		if targetNodePath != "" {
			dir, base := filepath.Split(targetNodePath)
			node := &tree.Node{Path: dir}
			_, nodeErr := inputFilter(ctx, node, "sel")
			if nodeErr != nil {
				log.Logger(ctx).Error("Filtering Input Node Parent", zap.Any("node", node), zap.Error(nodeErr))
				return nodeErr
			}
			targetNodePath = node.Path + "/" + base
		}

		log.Logger(ctx).Debug("Creating copy/move job", zap.Any("pathes", selectedPathes), zap.String("target", targetNodePath))

		job := &jobs.Job{
			ID:             "copy-move-" + jobUuid,
			Owner:          userName,
			Label:          "Compressing selection",
			HasProgress:    false,
			Stoppable:      true,
			Inactive:       false,
			MaxConcurrency: 1,
			AutoStart:      true,
			Actions: []*jobs.Action{
				{
					ID: "actions.tree.copymove",
					Parameters: map[string]string{
						"type":      taskType,
						"target":    targetNodePath,
						"recursive": "true",
						"create":    "true",
					},
					NodesSelector: &jobs.NodesSelector{
						Collect: true,
						Pathes:  selectedPathes,
					},
				},
			},
		}

		_, er := s.JobsClient.PutJob(ctx, &jobs.PutJobRequest{Job: job})
		return er

	})

	result := make(map[string]bool, 1)
	if err == nil {
		result["success"] = true
	} else {
		result["success"] = false
	}
	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}
	encoded, e := json.Marshal(result)
	if e != nil {
		return e
	}
	rsp.Body = string(encoded)
	return nil

}

func (s *Jobs) Clear(ctx context.Context, req *api.Request, rsp *api.Response) error {

	status := jobs.TaskStatus_Any
	if filter, ok := req.GetGet()["filter"]; ok {
		intVal, _ := strconv.ParseInt(filter.Values[0], 10, 32)
		status = jobs.TaskStatus(intVal)
	}

	// List all Jobs w. tasks
	streamer, err := s.JobsClient.ListJobs(ctx, &jobs.ListJobsRequest{
		LoadTasks: status,
	})
	if err != nil {
		return err
	}

	marshaler := &jsonpb.Marshaler{}
	//results := []*jobs.Job{}
	results := []interface{}{}
	for {
		resp, err := streamer.Recv()
		if err != nil {
			break
		}
		if resp == nil {
			continue
		}

		_, delE := s.JobsClient.DeleteJob(ctx, &jobs.DeleteJobRequest{JobID: resp.Job.ID})
		if delE == nil {
			s, _ := marshaler.MarshalToString(resp.Job)
			var rebuilt interface{}
			json.Unmarshal([]byte(s), &rebuilt)
			results = append(results, rebuilt)
		} else {
			results = append(results, map[string]string{"Error while deleting": resp.Job.ID})
		}

	}

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}

	encoded, e := json.Marshal(results)
	if e != nil {
		return e
	}
	rsp.Body = string(encoded)

	return nil

}

func (s *Jobs) List(ctx context.Context, req *api.Request, rsp *api.Response) error {

	status := jobs.TaskStatus_Any
	if filter, ok := req.GetGet()["filter"]; ok {
		intVal, _ := strconv.ParseInt(filter.Values[0], 10, 32)
		status = jobs.TaskStatus(intVal)
	}

	// List all Jobs w. tasks
	streamer, err := s.JobsClient.ListJobs(ctx, &jobs.ListJobsRequest{
		LoadTasks: status,
	})
	if err != nil {
		return err
	}

	marshaler := &jsonpb.Marshaler{}
	//results := []*jobs.Job{}
	results := []interface{}{}
	for {
		resp, err := streamer.Recv()
		if err != nil {
			break
		}
		if resp == nil {
			continue
		}

		//results = append(results, resp.Job)

		s, _ := marshaler.MarshalToString(resp.Job)
		var rebuilt interface{}
		json.Unmarshal([]byte(s), &rebuilt)
		results = append(results, rebuilt)

	}

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}

	encoded, e := json.Marshal(results)
	if e != nil {
		return e
	}
	rsp.Body = string(encoded)

	return nil

}

func apiBuilder(service micro.Service) interface{} {
	return &Jobs{
		JobsClient: jobs.NewJobServiceClient(common.SERVICE_JOBS, service.Client()),
		Router:     views.NewStandardRouter(false, true),
	}
}

// Starts the API
// Then Start micro --client=grpc api --namespace="pydio.service.api"
// Then call e.g. http://localhost:8080/jobs/list"
func NewJobsApiService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(apiBuilder, micro.Name(common.SERVICE_API_NAMESPACE_+"jobs"))
	return srv, nil

}
