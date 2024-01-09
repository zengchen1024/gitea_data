package access

import (
	"context"

	"code.gitea.io/gitea/models/organization"
	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

type Permission struct {
	AccessMode perm_model.AccessMode
	Units      []*repo_model.RepoUnit
	UnitsMode  map[unit.Type]perm_model.AccessMode
}

// GetUserRepoPermission returns the user permissions to the repository
func GetUserRepoPermission(ctx context.Context, repo *repo_model.Repository, user *user_model.User) (Permission, error) {
	var perm Permission
	if log.IsTrace() {
		defer func() {
			if user == nil {
				log.Trace("Permission Loaded for anonymous user in %-v:\nPermissions: %-+v",
					repo,
					perm)
				return
			}
			log.Trace("Permission Loaded for %-v in %-v:\nPermissions: %-+v",
				user,
				repo,
				perm)
		}()
	}

	// anonymous user visit private repo.
	// TODO: anonymous user visit public unit of private repo???
	if user == nil && repo.IsPrivate {
		perm.AccessMode = perm_model.AccessModeNone
		return perm, nil
	}

	var isCollaborator bool
	var err error
	if user != nil {
		isCollaborator, err = repo_model.IsCollaborator(ctx, repo.ID, user.ID)
		if err != nil {
			return perm, err
		}
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return perm, err
	}

	// Prevent strangers from checking out public repo of private organization/users
	// Allow user if they are collaborator of a repo within a private user or a private organization but not a member of the organization itself
	if !organization.HasOrgOrUserVisible(ctx, repo.Owner, user) && !isCollaborator {
		perm.AccessMode = perm_model.AccessModeNone
		return perm, nil
	}

	if err := repo.LoadUnits(ctx); err != nil {
		return perm, err
	}

	perm.Units = repo.Units

	// anonymous visit public repo
	if user == nil {
		perm.AccessMode = perm_model.AccessModeRead
		return perm, nil
	}

	// Admin or the owner has super access to the repository
	if user.IsAdmin || user.ID == repo.OwnerID {
		perm.AccessMode = perm_model.AccessModeOwner
		return perm, nil
	}

	// plain user
	perm.AccessMode, err = accessLevel(ctx, user, repo)
	if err != nil {
		return perm, err
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return perm, err
	}
	if !repo.Owner.IsOrganization() {
		return perm, nil
	}

	perm.UnitsMode = make(map[unit.Type]perm_model.AccessMode)

	// Collaborators on organization
	if isCollaborator {
		for _, u := range repo.Units {
			perm.UnitsMode[u.Type] = perm.AccessMode
		}
	}

	// get units mode from teams
	teams, err := organization.GetUserRepoTeams(ctx, repo.OwnerID, user.ID, repo.ID)
	if err != nil {
		return perm, err
	}

	// if user in an owner team
	for _, team := range teams {
		if team.AccessMode >= perm_model.AccessModeAdmin {
			perm.AccessMode = perm_model.AccessModeOwner
			perm.UnitsMode = nil
			return perm, nil
		}
	}

	for _, u := range repo.Units {
		var found bool
		for _, team := range teams {
			teamMode := team.UnitAccessMode(ctx, u.Type)
			if teamMode > perm_model.AccessModeNone {
				m := perm.UnitsMode[u.Type]
				if m < teamMode {
					perm.UnitsMode[u.Type] = teamMode
				}
				found = true
			}
		}

		// for a public repo on an organization, a non-restricted user has read permission on non-team defined units.
		if !found && !repo.IsPrivate && !user.IsRestricted {
			if _, ok := perm.UnitsMode[u.Type]; !ok {
				perm.UnitsMode[u.Type] = perm_model.AccessModeRead
			}
		}
	}

	// remove no permission units
	perm.Units = make([]*repo_model.RepoUnit, 0, len(repo.Units))
	for t := range perm.UnitsMode {
		for _, u := range repo.Units {
			if u.Type == t {
				perm.Units = append(perm.Units, u)
			}
		}
	}

	return perm, err
}

func (p *Permission) CanAccess(mode perm_model.AccessMode, unitType unit.Type) bool {
	return p.UnitAccessMode(unitType) >= mode
}

// UnitAccessMode returns current user accessmode to the specify unit of the repository
func (p *Permission) UnitAccessMode(unitType unit.Type) perm_model.AccessMode {
	if p.UnitsMode == nil {
		for _, u := range p.Units {
			if u.Type == unitType {
				return p.AccessMode
			}
		}
		return perm_model.AccessModeNone
	}
	return p.UnitsMode[unitType]
}
