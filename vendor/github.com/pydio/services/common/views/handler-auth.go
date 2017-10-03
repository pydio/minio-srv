package views

import (
	//	"errors"
	"github.com/pydio/services/common/log"
	"strings"

	"github.com/pydio/services/common/auth"
	"github.com/pydio/services/common/utils"

	"github.com/pydio/services/common/proto/idm"
	"golang.org/x/net/context"
)

type AuthHandler struct {
	AbstractHandler
}

func NewAuthHandler(adminView bool) *AuthHandler {
	a := &AuthHandler{}
	a.CtxWrapper = func(ctx context.Context) (context.Context, error) {
		if adminView {
			return context.WithValue(ctx, ctxAdminContextKey{}, true), nil
		}
		workspaces, err := loadWorkspaces(ctx)
		if err != nil {
			return ctx, err
		}
		ctx = context.WithValue(ctx, ctxUserWorkspacesKey{}, workspaces)
		return ctx, nil
	}
	return a
}

func loadWorkspaces(ctx context.Context) (workspaces map[string]*idm.Workspace, err error) {

	workspaces = make(map[string]*idm.Workspace)

	claims, ok := ctx.Value(auth.PYDIO_CONTEXT_CLAIMS_KEY).(auth.Claims)
	if !ok {
		log.Logger(ctx).Debug("No Claims in Context, workspaces will be empty - should be anonymous user")
		return workspaces, nil
		//return nil, errors.New("Could not retrieve claims from context")
	}

	roles := utils.GetRoles(strings.Split(claims.Roles, ","))

	aclsRead := utils.GetACLsForRoles(roles, utils.ACL_READ)
	aclsDeny := utils.GetACLsForRoles(roles, utils.ACL_DENY)

	idmWorkspaces := utils.GetWorkspacesForACLs(aclsRead, aclsDeny)

	for _, workspace := range idmWorkspaces {
		workspaces[workspace.UUID] = workspace
	}

	return workspaces, nil
}
