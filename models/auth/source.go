// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/builder"
	"xorm.io/xorm/convert"
)

// Type represents an login type.
type Type int

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

// Source represents an external way for authorizing users.
type Source struct {
	ID            int64 `xorm:"pk autoincr"`
	Type          Type
	Name          string             `xorm:"UNIQUE"`
	IsActive      bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	IsSyncEnabled bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	Cfg           convert.Conversion `xorm:"TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

type FindSourcesOptions struct {
	IsActive  util.OptionalBool
	LoginType Type
}

func (opts FindSourcesOptions) ToConds() builder.Cond {
	conds := builder.NewCond()
	if !opts.IsActive.IsNone() {
		conds = conds.And(builder.Eq{"is_active": opts.IsActive.IsTrue()})
	}
	if opts.LoginType != NoType {
		conds = conds.And(builder.Eq{"`type`": opts.LoginType})
	}
	return conds
}

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
