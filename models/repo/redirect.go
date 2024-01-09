// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
)

type Redirect = repo_model.Redirect
type ErrRedirectNotExist = repo_model.ErrRedirectNotExist

// LookupRedirect look up if a repository has a redirect name
func LookupRedirect(ownerID int64, repoName string) (int64, error) {
	repoName = strings.ToLower(repoName)
	redirect := &Redirect{OwnerID: ownerID, LowerName: repoName}
	if has, err := db.GetEngine(db.DefaultContext).Get(redirect); err != nil {
		return 0, err
	} else if !has {
		return 0, ErrRedirectNotExist{OwnerID: ownerID, RepoName: repoName}
	}
	return redirect.RedirectRepoID, nil
}
