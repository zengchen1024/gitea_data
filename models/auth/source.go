// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	"code.gitea.io/gitea/models/db"
	models_auth "code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

type Type = models_auth.Type

// Note: new type must append to the end of list to maintain compatibility.
const (
	NoType Type = iota
	Plain       // 1
	LDAP        // 2
	SMTP        // 3
	PAM         // 4
	DLDAP       // 5
	OAuth2      // 6
	SSPI        // 7
)

type Source = models_auth.Source

type FindSourcesOptions = models_auth.FindSourcesOptions

// FindSources returns a slice of login sources found in DB according to given conditions.
func FindSources(ctx context.Context, opts FindSourcesOptions) ([]*Source, error) {
	auths := make([]*Source, 0, 6)
	return auths, db.GetEngine(ctx).Where(opts.ToConds()).Find(&auths)
}

// IsSSPIEnabled returns true if there is at least one activated login
// source of type LoginSSPI
func IsSSPIEnabled(ctx context.Context) bool {
	if !db.HasEngine {
		return false
	}
	sources, err := FindSources(ctx, FindSourcesOptions{
		IsActive:  util.OptionalBoolTrue,
		LoginType: SSPI,
	})
	if err != nil {
		log.Error("ActiveSources: %v", err)
		return false
	}
	return len(sources) > 0
}
