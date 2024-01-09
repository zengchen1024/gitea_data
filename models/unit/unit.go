// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unit

import (
	models_unit "code.gitea.io/gitea/models/unit"
)

// Type is Unit's Type
type Type = models_unit.Type

// Enumerate all the unit types
const (
	TypeInvalid         Type = iota // 0 invalid
	TypeCode                        // 1 code
	TypeIssues                      // 2 issues
	TypePullRequests                // 3 PRs
	TypeReleases                    // 4 Releases
	TypeWiki                        // 5 Wiki
	TypeExternalWiki                // 6 ExternalWiki
	TypeExternalTracker             // 7 ExternalTracker
	TypeProjects                    // 8 Kanban board
	TypePackages                    // 9 Packages
	TypeActions                     // 10 Actions
)
