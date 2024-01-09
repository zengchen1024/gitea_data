package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util"

	"github.com/openmerlin/gitea_data/routers/models/git/internal"
)

// Command represents a command with its subcommands or arguments.
type Command struct {
	prog             string
	args             []string
	parentContext    context.Context
	desc             string
	globalArgsLength int
	brokenArgs       []string
}

// DefaultLocale is the default LC_ALL to run git commands in.
const DefaultLocale = "C"

func (c *Command) String() string {
	return c.toString(false)
}

func (c *Command) toString(sanitizing bool) string {
	// WARNING: this function is for debugging purposes only. It's much better than old code (which only joins args with space),
	// It's impossible to make a simple and 100% correct implementation of argument quoting for different platforms.
	debugQuote := func(s string) string {
		if strings.ContainsAny(s, " `'\"\t\r\n") {
			return fmt.Sprintf("%q", s)
		}
		return s
	}
	a := make([]string, 0, len(c.args)+1)
	a = append(a, debugQuote(c.prog))
	for _, arg := range c.args {
		if sanitizing && (strings.Contains(arg, "://") && strings.Contains(arg, "@")) {
			a = append(a, debugQuote(util.SanitizeCredentialURLs(arg)))
		} else {
			a = append(a, debugQuote(arg))
		}
	}
	return strings.Join(a, " ")
}

type TrustedCmdArgs []internal.CmdArg

var (
	// globalCommandArgs global command args for external package setting
	globalCommandArgs TrustedCmdArgs

	// defaultCommandExecutionTimeout default command execution timeout duration
	defaultCommandExecutionTimeout = 360 * time.Second
)

// NewCommand creates and returns a new Git Command based on given command and arguments.
// Each argument should be safe to be trusted. User-provided arguments should be passed to AddDynamicArguments instead.
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

func (c *Command) AddArguments(args ...internal.CmdArg) *Command {
	for _, arg := range args {
		c.args = append(c.args, string(arg))
	}
	return c
}

func (c *Command) SetDescription(desc string) *Command {
	c.desc = desc
	return c
}

// isSafeArgumentValue checks if the argument is safe to be used as a value (not an option)
func isSafeArgumentValue(s string) bool {
	return s == "" || s[0] != '-'
}

func (c *Command) AddDynamicArguments(args ...string) *Command {
	for _, arg := range args {
		if !isSafeArgumentValue(arg) {
			c.brokenArgs = append(c.brokenArgs, arg)
		}
	}
	if len(c.brokenArgs) != 0 {
		return c
	}
	c.args = append(c.args, args...)
	return c
}

func commonBaseEnvs() []string {
	// at the moment, do not set "GIT_CONFIG_NOSYSTEM", users may have put some configs like "receive.certNonceSeed" in it
	envs := []string{
		"HOME=" + HomeDir(),        // make Gitea use internal git config only, to prevent conflicts with user's git config
		"GIT_NO_REPLACE_OBJECTS=1", // ignore replace references (https://git-scm.com/docs/git-replace)
	}

	// some environment variables should be passed to git command
	passThroughEnvKeys := []string{
		"GNUPGHOME", // git may call gnupg to do commit signing
	}
	for _, key := range passThroughEnvKeys {
		if val, ok := os.LookupEnv(key); ok {
			envs = append(envs, key+"="+val)
		}
	}
	return envs
}

// CommonGitCmdEnvs returns the common environment variables for a "git" command.
func CommonGitCmdEnvs() []string {
	return append(commonBaseEnvs(), []string{
		"LC_ALL=" + DefaultLocale,
		"GIT_TERMINAL_PROMPT=0", // avoid prompting for credentials interactively, supported since git v2.3
	}...)
}

var ErrBrokenCommand = errors.New("git command is broken")

// Run runs the command with the RunOpts
func (c *Command) Run(opts *RunOpts) error {
	if len(c.brokenArgs) != 0 {
		log.Error("git command is broken: %s, broken args: %s", c.String(), strings.Join(c.brokenArgs, " "))
		return ErrBrokenCommand
	}
	if opts == nil {
		opts = &RunOpts{}
	}

	// We must not change the provided options
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultCommandExecutionTimeout
	}

	if len(opts.Dir) == 0 {
		log.Debug("git.Command.Run: %s", c)
	} else {
		log.Debug("git.Command.RunDir(%s): %s", opts.Dir, c)
	}

	desc := c.desc
	if desc == "" {
		if opts.Dir == "" {
			desc = fmt.Sprintf("git: %s", c.toString(true))
		} else {
			desc = fmt.Sprintf("git(dir:%s): %s", opts.Dir, c.toString(true))
		}
	}

	var ctx context.Context
	var cancel context.CancelFunc
	var finished context.CancelFunc

	if opts.UseContextTimeout {
		ctx, cancel, finished = process.GetManager().AddContext(c.parentContext, desc)
	} else {
		ctx, cancel, finished = process.GetManager().AddContextTimeout(c.parentContext, timeout, desc)
	}
	defer finished()

	startTime := time.Now()

	cmd := exec.CommandContext(ctx, c.prog, c.args...)
	if opts.Env == nil {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = opts.Env
	}

	process.SetSysProcAttribute(cmd)
	cmd.Env = append(cmd.Env, CommonGitCmdEnvs()...)
	cmd.Dir = opts.Dir
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	cmd.Stdin = opts.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}

	if opts.PipelineFunc != nil {
		err := opts.PipelineFunc(ctx, cancel)
		if err != nil {
			cancel()
			_ = cmd.Wait()
			return err
		}
	}

	err := cmd.Wait()
	elapsed := time.Since(startTime)
	if elapsed > time.Second {
		log.Debug("slow git.Command.Run: %s (%s)", c, elapsed)
	}

	if err != nil && ctx.Err() != context.DeadlineExceeded {
		return err
	}

	return ctx.Err()
}

// RunOpts represents parameters to run the command. If UseContextTimeout is specified, then Timeout is ignored.
type RunOpts struct {
	Env               []string
	Timeout           time.Duration
	UseContextTimeout bool

	// Dir is the working dir for the git command, however:
	// FIXME: this could be incorrect in many cases, for example:
	// * /some/path/.git
	// * /some/path/.git/gitea-data/data/repositories/user/repo.git
	// If "user/repo.git" is invalid/broken, then running git command in it will use "/some/path/.git", and produce unexpected results
	// The correct approach is to use `--git-dir" global argument
	Dir string

	Stdout, Stderr io.Writer

	// Stdin is used for passing input to the command
	// The caller must make sure the Stdin writer is closed properly to finish the Run function.
	// Otherwise, the Run function may hang for long time or forever, especially when the Git's context deadline is not the same as the caller's.
	// Some common mistakes:
	// * `defer stdinWriter.Close()` then call `cmd.Run()`: the Run() would never return if the command is killed by timeout
	// * `go { case <- parentContext.Done(): stdinWriter.Close() }` with `cmd.Run(DefaultTimeout)`: the command would have been killed by timeout but the Run doesn't return until stdinWriter.Close()
	// * `go { if stdoutReader.Read() err != nil: stdinWriter.Close() }` with `cmd.Run()`: the stdoutReader may never return error if the command is killed by timeout
	// In the future, ideally the git module itself should have full control of the stdin, to avoid such problems and make it easier to refactor to a better architecture.
	Stdin io.Reader

	PipelineFunc func(context.Context, context.CancelFunc) error
}
