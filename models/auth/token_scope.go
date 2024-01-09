// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/perm"
)

const (
	NoAccess AccessTokenScopeLevel = iota
	Read
	Write
)

type AccessTokenScopeLevel = auth_model.AccessTokenScopeLevel

// GetScopeLevelFromAccessMode converts permission access mode to scope level
func GetScopeLevelFromAccessMode(mode perm.AccessMode) AccessTokenScopeLevel {
	switch mode {
	case perm.AccessModeNone:
		return NoAccess
	case perm.AccessModeRead:
		return Read
	case perm.AccessModeWrite:
		return Write
	case perm.AccessModeAdmin:
		return Write
	case perm.AccessModeOwner:
		return Write
	default:
		return NoAccess
	}
}
