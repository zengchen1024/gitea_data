// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
)

type Repository = repo_model.Repository
type ErrRepoNotExist = repo_model.ErrRepoNotExist

// GetRepositoryByName returns the repository by given name under user if exists.
func GetRepositoryByName(ownerID int64, name string) (*Repository, error) {
	repo := &Repository{
		OwnerID:   ownerID,
		LowerName: strings.ToLower(name),
	}
	has, err := db.GetEngine(db.DefaultContext).Get(repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{0, ownerID, "", name}
	}
	return repo, err
}

// IsErrRepoNotExist checks if an error is a ErrRepoNotExist.
func IsErrRepoNotExist(err error) bool {
	_, ok := err.(ErrRepoNotExist)
	return ok
}

// RepoPath returns repository path by given user and repository name.
func RepoPath(userName, repoName string) string { //revive:disable-line:exported
	return filepath.Join(user_model.UserPath(userName), strings.ToLower(repoName)+".git")
}

// GetRepositoryByOwnerAndName returns the repository by given owner name and repo name
func GetRepositoryByOwnerAndName(ctx context.Context, ownerName, repoName string) (*Repository, error) {
	var repo Repository
	has, err := db.GetEngine(ctx).Table("repository").Select("repository.*").
		Join("INNER", "`user`", "`user`.id = repository.owner_id").
		Where("repository.lower_name = ?", strings.ToLower(repoName)).
		And("`user`.lower_name = ?", strings.ToLower(ownerName)).
		Get(&repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{0, 0, ownerName, repoName}
	}
	return &repo, nil
}

func ComposeSSHCloneURL(ownerName, repoName string) string {
	sshUser := setting.SSH.User

	// if we have a ipv6 literal we need to put brackets around it
	// for the git cloning to work.
	sshDomain := setting.SSH.Domain
	ip := net.ParseIP(setting.SSH.Domain)
	if ip != nil && ip.To4() == nil {
		sshDomain = "[" + setting.SSH.Domain + "]"
	}

	if setting.SSH.Port != 22 {
		return fmt.Sprintf("ssh://%s@%s/%s/%s.git", sshUser,
			net.JoinHostPort(setting.SSH.Domain, strconv.Itoa(setting.SSH.Port)),
			url.PathEscape(ownerName),
			url.PathEscape(repoName))
	}
	if setting.Repository.UseCompatSSHURI {
		return fmt.Sprintf("ssh://%s@%s/%s/%s.git", sshUser, sshDomain, url.PathEscape(ownerName), url.PathEscape(repoName))
	}
	return fmt.Sprintf("%s@%s:%s/%s.git", sshUser, sshDomain, url.PathEscape(ownerName), url.PathEscape(repoName))
}

// ComposeHTTPSCloneURL returns HTTPS clone URL based on given owner and repository name.
func ComposeHTTPSCloneURL(owner, repo string) string {
	return fmt.Sprintf("%s%s/%s.git", setting.AppURL, url.PathEscape(owner), url.PathEscape(repo))
}
