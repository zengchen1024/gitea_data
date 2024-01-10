// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	notify_service "code.gitea.io/gitea/services/notify"
)

// PushCreateRepo creates a repository when a new repository is pushed to an appropriate namespace
func PushCreateRepo(ctx context.Context, authUser, owner *user_model.User, repoName string) (*repo_model.Repository, error) {
	if !authUser.IsAdmin {
		if owner.IsOrganization() {
			if ok, err := organization.CanCreateOrgRepo(ctx, owner.ID, authUser.ID); err != nil {
				return nil, err
			} else if !ok {
				return nil, fmt.Errorf("cannot push-create repository for org")
			}
		} else if authUser.ID != owner.ID {
			return nil, fmt.Errorf("cannot push-create repository for another user")
		}
	}

	repo, err := CreateRepository(ctx, authUser, owner, CreateRepoOptions{
		Name:      repoName,
		IsPrivate: setting.Repository.DefaultPushCreatePrivate,
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// CreateRepository creates a repository for the user/organization.
func CreateRepository(ctx context.Context, doer, owner *user_model.User, opts CreateRepoOptions) (*repo_model.Repository, error) {
	repo, err := CreateRepositoryDirectly(ctx, doer, owner, opts)
	if err != nil {
		// No need to rollback here we should do this in CreateRepository...
		return nil, err
	}

	notify_service.CreateRepository(ctx, doer, owner, repo)

	return repo, nil
}

// UpdateRepository updates a repository
func UpdateRepository(ctx context.Context, repo *repo_model.Repository, visibilityChanged bool) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = repo_module.UpdateRepository(ctx, repo, visibilityChanged); err != nil {
		return fmt.Errorf("updateRepository: %w", err)
	}

	return committer.Commit()
}
