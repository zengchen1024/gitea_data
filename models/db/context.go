// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import "context"

// DefaultContext is the default context to run xorm queries in
// will be overwritten by Init with HammerContext
var DefaultContext context.Context
