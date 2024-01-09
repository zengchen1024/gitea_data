// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package perm

import (
	models_perm "code.gitea.io/gitea/models/perm"
)

type AccessMode = models_perm.AccessMode

const (
	// AccessModeNone no access
	AccessModeNone AccessMode = iota // 0
	// AccessModeRead read access
	AccessModeRead // 1
	// AccessModeWrite write access
	AccessModeWrite // 2
	// AccessModeAdmin admin access
	AccessModeAdmin // 3
	// AccessModeOwner owner access
	AccessModeOwner // 4
)
