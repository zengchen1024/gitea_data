// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// ComposeGoGetImport returns go-get-import meta content.
func ComposeGoGetImport(owner, repo string) string {
	/// setting.AppUrl is guaranteed to be parse as url
	appURL, _ := url.Parse(setting.AppURL)

	return path.Join(appURL.Host, setting.AppSubURL, url.PathEscape(owner), url.PathEscape(repo))
}

// EarlyResponseForGoGetMeta responses appropriate go-get meta with status 200
// if user does not have actual access to the requested repository,
// or the owner or repository does not exist at all.
// This is particular a workaround for "go get" command which does not respect
// .netrc file.
func EarlyResponseForGoGetMeta(ctx *Context) {
	username := ctx.Params(":username")
	reponame := strings.TrimSuffix(ctx.Params(":reponame"), ".git")
	if username == "" || reponame == "" {
		ctx.PlainText(http.StatusBadRequest, "invalid repository path")
		return
	}

	var cloneURL string
	if setting.Repository.GoGetCloneURLProtocol == "ssh" {
		cloneURL = repo_model.ComposeSSHCloneURL(username, reponame)
	} else {
		cloneURL = repo_model.ComposeHTTPSCloneURL(username, reponame)
	}
	goImportContent := fmt.Sprintf("%s git %s", ComposeGoGetImport(username, reponame), cloneURL)
	htmlMeta := fmt.Sprintf(`<meta name="go-import" content="%s">`, html.EscapeString(goImportContent))
	ctx.PlainText(http.StatusOK, htmlMeta)
}

// RedirectToRepo redirect to a differently-named repository
func RedirectToRepo(ctx *Base, redirectRepoID int64) {
	ownerName := ctx.Params(":username")
	previousRepoName := ctx.Params(":reponame")

	repo, err := repo_model.GetRepositoryByID(ctx, redirectRepoID)
	if err != nil {
		log.Error("GetRepositoryByID: %v", err)
		ctx.Error(http.StatusInternalServerError, "GetRepositoryByID")
		return
	}

	redirectPath := strings.Replace(
		ctx.Req.URL.EscapedPath(),
		url.PathEscape(ownerName)+"/"+url.PathEscape(previousRepoName),
		url.PathEscape(repo.OwnerName)+"/"+url.PathEscape(repo.Name),
		1,
	)
	if ctx.Req.URL.RawQuery != "" {
		redirectPath += "?" + ctx.Req.URL.RawQuery
	}
	ctx.Redirect(path.Join(setting.AppSubURL, redirectPath), http.StatusTemporaryRedirect)
}
