package utils

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	grpc "github.com/micro/go-grpc"
	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/config"
	"github.com/pydio/services/common/proto/acl"
	"github.com/pydio/services/common/proto/idm"
	service "github.com/pydio/services/common/service/proto"

	"github.com/micro/config-srv/proto/config"
)

var (
	ACL_READ  = &acl.ACLAction{Name: "read", Value: "1"}
	ACL_WRITE = &acl.ACLAction{Name: "write", Value: "1"}
	ACL_DENY  = &acl.ACLAction{Name: "deny", Value: "1"}

	configClient    go_micro_srv_config_config.ConfigClient
	roleClient      idm.RoleServiceClient
	aclClient       acl.ACLServiceClient
	workspaceClient idm.WorkspaceServiceClient
)

func init() {

	configService := grpc.NewService(micro.Name(common.SERVICE_CONFIG + ".client"))
	configClient = go_micro_srv_config_config.NewConfigClient(common.SERVICE_CONFIG, configService.Client())

	roleService := grpc.NewService(micro.Name(common.SERVICE_ROLE + ".client"))
	roleClient = idm.NewRoleServiceClient(common.SERVICE_ROLE, roleService.Client())

	aclService := grpc.NewService(micro.Name(common.SERVICE_ACL + ".client"))
	aclClient = acl.NewACLServiceClient(common.SERVICE_ACL, aclService.Client())

	workspaceService := grpc.NewService(micro.Name(common.SERVICE_WORKSPACE + ".client"))
	workspaceClient = idm.NewWorkspaceServiceClient(common.SERVICE_WORKSPACE, workspaceService.Client())
}

// GetConfigsForService name or regexp in argument
func GetConfigsForService(name string) (config.Map, error) {

	response, err := configClient.Search(context.Background(), &go_micro_srv_config_config.SearchRequest{
		Id:    "services/" + name,
		Limit: 1,
	})

	if err != nil {
		return nil, err
	}

	if len(response.Configs) != 1 {
		return nil, errors.New("Wrong configuration")
	}

	var data config.Map
	if err := json.Unmarshal([]byte(response.Configs[0].ChangeSet.Data), &data); err != nil {
		return nil, err
	}

	return data, nil
}

func GetRolesForUser(name string) []*idm.Role {

	var roles []*idm.Role

	query, _ := ptypes.MarshalAny(&idm.RoleSingleQuery{
		Name: []string{name},
	})

	if stream, err := roleClient.SearchRole(context.Background(), &idm.SearchRoleRequest{
		Query: &service.Query{
			SubQueries: []*any.Any{query},
		},
	}); err != nil {
		return nil
	} else {

		defer stream.Close()

		for {
			response, err := stream.Recv()

			if err != nil {
				break
			}

			roles = append(roles, response.GetRole())
		}
	}

	return roles
}

// GetRoles Objects from a list of role names
func GetRoles(names []string) []*idm.Role {
	var roles []*idm.Role

	// First we retrieve the roleIDs from the role names
	query, _ := ptypes.MarshalAny(&idm.RoleSingleQuery{Name: names})
	stream, err := roleClient.SearchRole(context.Background(), &idm.SearchRoleRequest{Query: &service.Query{SubQueries: []*any.Any{query}}})

	if err != nil {
		return nil
	}

	defer stream.Close()

	for {
		response, err := stream.Recv()

		if err != nil {
			break
		}

		roles = append(roles, response.GetRole())
	}

	return roles
}

func GetACLsForRoles(roles []*idm.Role, actions ...*acl.ACLAction) []*acl.ACL {

	var acls []*acl.ACL

	// First we retrieve the roleIDs from the role names
	var roleIDs []string
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
	}

	query, _ := ptypes.MarshalAny(&acl.ACLSingleQuery{RoleIDs: roleIDs, Actions: actions})
	stream, err := aclClient.SearchACL(context.Background(), &acl.SearchACLRequest{
		Query: &service.Query{
			SubQueries: []*any.Any{query},
		},
	})

	if err != nil {
		return nil
	}

	defer stream.Close()

	for {
		response, err := stream.Recv()

		if err != nil {
			break
		}

		acls = append(acls, response.GetACL())
	}

	return acls
}

// Should return a workspace, but returning nodes now until we handle multiple nodes per workspace
func GetWorkspacesForACLs(read []*acl.ACL, deny []*acl.ACL) []*idm.Workspace {

	var workspaces []*idm.Workspace

	workspaceIDs := make(map[string]map[string]string)

	for _, aclRow := range read {
		workspaceID, nodeID := aclRow.GetWorkspaceID(), aclRow.GetNodeID()
		if workspaceID == "" || nodeID == "" {
			continue
		}
		var crtIds map[string]string
		var ok bool
		if crtIds, ok = workspaceIDs[workspaceID]; !ok {
			crtIds = make(map[string]string)
		}
		crtIds[aclRow.NodeID] = aclRow.NodeID
		workspaceIDs[workspaceID] = crtIds
	}

	for _, aclRow := range deny {
		workspaceID := aclRow.GetWorkspaceID()
		if workspaceID != "" {
			delete(workspaceIDs, workspaceID)
		}
	}

	var queries []*any.Any
	for workspaceID := range workspaceIDs {
		query, _ := ptypes.MarshalAny(&idm.WorkspaceSingleQuery{Uuid: workspaceID})
		queries = append(queries, query)
	}

	stream, err := workspaceClient.SearchWorkspace(context.Background(), &idm.SearchWorkspaceRequest{
		Query: &service.Query{
			SubQueries: queries,
			Operation:  service.OperationType_OR,
		},
	})

	if err != nil {
		return nil
	}

	defer stream.Close()

	for {
		response, err := stream.Recv()

		if err != nil {
			break
		}

		ws := response.GetWorkspace()
		for nodeUuid := range workspaceIDs[ws.UUID] {
			ws.RootNodes = append(ws.RootNodes, nodeUuid)
		}
		workspaces = append(workspaces, ws)
	}

	return workspaces
}
