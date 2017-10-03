package acl

import (
	"bytes"

	"github.com/micro/go-micro/errors"
	"github.com/olekukonko/tablewriter"
	"github.com/pydio/services/common"
)

type ACLSet struct {
	acls            []*ACL
	mergedByActions map[string]*ACL
}

func (set *ACLSet) InitFromResponse(resp ACLService_SearchACLClient) {

	defer resp.Close()

	for {
		searchACLResponse, e := resp.Recv()
		if searchACLResponse == nil || e != nil {
			break
		}
		set.acls = append(set.acls, searchACLResponse.ACL)
	}

}

func (set *ACLSet) Append(acl *ACL) {
	set.acls = append(set.acls, acl)
}

func (set *ACLSet) ByActions() (map[string]*ACL, error) {
	if set.mergedByActions == nil {
		return nil, errors.BadRequest(common.SERVICE_ACL, "Please run set.Merge() before calling ByActions()")
	}
	return set.mergedByActions, nil
}

func (set *ACLSet) String() string {

	stringBuffer := bytes.NewBufferString("\n")
	table := tablewriter.NewWriter(stringBuffer)
	table.SetHeader([]string{"Role Id", "Node Id", "Action Name", "Action Value", "Workspace"})

	for _, v := range set.acls {
		table.Append([]string{v.RoleID, v.NodeID, v.Action.Name, v.Action.Value, v.WorkspaceID})
	}
	table.Render() // Send output
	return stringBuffer.String()

}

// This function expect ACL's in set are already
// consistent on a same Node ID and a same Workspace ID
func (set *ACLSet) Merge(rolesList map[int]string, orderedKeys []int) (acl *ACLSet) {

	// If no ACL's are registered, directly return
	if len(set.acls) == 0 {
		return set
	}

	// Workspace ID => ACL's
	out := &ACLSet{}
	set.mergedByActions = make(map[string]*ACL)
	for _, k := range orderedKeys {
		roleId := rolesList[k]
		for _, roleACL := range set.aclsForRole(roleId) {
			if existing, ok := set.mergedByActions[roleACL.Action.Name]; ok == true {
				// If it's deny, ignore current value
				if existing.Action.Name == "deny" && existing.Action.Value == "1" {
					continue
				}
			}
			// Create new ACL
			newACL := &ACL{
				RoleID:      "%%MERGED%%",
				NodeID:      roleACL.NodeID,
				Action:      roleACL.Action,
				WorkspaceID: roleACL.WorkspaceID,
			}
			set.mergedByActions[roleACL.Action.Name] = newACL
		}
	}
	for _, r := range set.mergedByActions {
		out.acls = append(out.acls, r)
	}
	return out
}

func (set *ACLSet) aclsForRole(roleId string) []*ACL {
	var out []*ACL
	for _, a := range set.acls {
		if a.RoleID == roleId {
			out = append(out, a)
		}
	}
	return out
}

func (set *ACLSet) GetActionValue(actionName string) string {

	for _, a := range set.acls {
		if a.Action.Name == actionName {
			return a.Action.Value
		}
	}
	return ""

}

/*func (set *ACLQuery) mergeACL(acl1 *ACL, acl2 *ACL) (output *ACL) {

	return &ACL{}
}*/
