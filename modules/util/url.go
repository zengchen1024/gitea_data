// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"net/url"
	"strings"
)

// PathEscapeSegments escapes segments of a path while not escaping forward slash
func PathEscapeSegments(path string) string {
	slice := strings.Split(path, "/")
	for index := range slice {
		slice[index] = url.PathEscape(slice[index])
	}
	escapedPath := strings.Join(slice, "/")
	return escapedPath
}
