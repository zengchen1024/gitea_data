package git

import (
	"context"

	"github.com/hashicorp/go-version"
)

const RequiredVersion = "2.0.0"

var (
	// GitExecutable is the command name of git
	// Could be updated to an absolute path while initialization
	GitExecutable = "git"

	// DefaultContext is the default context to run git commands in, must be initialized by git.InitXxx
	DefaultContext context.Context

	// SupportProcReceive version >= 2.29.0
	SupportProcReceive bool

	gitVersion *version.Version
)
