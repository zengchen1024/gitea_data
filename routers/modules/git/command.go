package git

import (
	"context"

	"github.com/openmerlin/gitea_data/routers/modules/git/internal" //nolint:depguard // only this file can use the internal type CmdArg, other files and packages should use AddXxx functions
)

type TrustedCmdArgs []internal.CmdArg

var (
	// globalCommandArgs global command args for external package setting
	globalCommandArgs TrustedCmdArgs
)

type Command struct {
	prog             string
	args             []string
	parentContext    context.Context
	desc             string
	globalArgsLength int
	brokenArgs       []string
}

func NewCommand(ctx context.Context, args ...internal.CmdArg) *Command {
	// Make an explicit copy of globalCommandArgs, otherwise append might overwrite it
	cargs := make([]string, 0, len(globalCommandArgs)+len(args))
	for _, arg := range globalCommandArgs {
		cargs = append(cargs, string(arg))
	}
	for _, arg := range args {
		cargs = append(cargs, string(arg))
	}
	return &Command{
		prog:             GitExecutable,
		args:             cargs,
		parentContext:    ctx,
		globalArgsLength: len(globalCommandArgs),
	}
}
