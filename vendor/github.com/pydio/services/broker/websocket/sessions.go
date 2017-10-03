package websocket

import (
	"context"
	"strings"

	"github.com/pydio/services/common/auth"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/utils"
	"go.uber.org/zap"
	"gopkg.in/olahol/melody.v1"
)

const SessionWorkspacesKey = "workspaces"

func UpdateSessionFromClaims(session *melody.Session, claims auth.Claims) {

	workspaces := make(map[string]*idm.Workspace)

	roles := utils.GetRoles(strings.Split(claims.Roles, ","))

	aclsRead := utils.GetACLsForRoles(roles, utils.ACL_READ)
	aclsDeny := utils.GetACLsForRoles(roles, utils.ACL_DENY)

	idmWorkspaces := utils.GetWorkspacesForACLs(aclsRead, aclsDeny)

	for _, workspace := range idmWorkspaces {
		workspaces[workspace.UUID] = workspace
	}

	log.Logger(context.Background()).Debug("Setting workspaces in session", zap.Any("workspaces", workspaces))
	session.Set(SessionWorkspacesKey, workspaces)

}

func ClearSession(session *melody.Session) {

	session.Set(SessionWorkspacesKey, nil)

}
