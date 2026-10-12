// Package launcher builds the commands for tools started against a target.
package launcher

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/session"
)

// Shell returns $SHELL, falling back to /bin/sh.
func Shell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

// Expand replaces the placeholders $SHELL, ${SHELL}, {garden}, {project},
// {seed}, {shoot} and {namespace} in every argument.
func Expand(argv []string, t session.Target, shell string) []string {
	r := strings.NewReplacer(
		"${SHELL}", shell,
		"$SHELL", shell,
		"{garden}", t.Garden,
		"{project}", t.Project,
		"{seed}", t.Seed,
		"{shoot}", t.Shoot,
		"{namespace}", t.Namespace,
	)
	out := make([]string, len(argv))
	for i, a := range argv {
		out[i] = r.Replace(a)
	}
	return out
}

// LookPath is exec.LookPath; tests replace it.
var LookPath = exec.LookPath

// Available reports whether the tool's executable can be found.
func Available(tool config.Tool) bool {
	if len(tool.Command) == 0 {
		return false
	}
	bin := Expand(tool.Command[:1], session.Target{}, Shell())[0]
	_, err := LookPath(bin)
	return err == nil
}

// Options control how a tool is started.
type Options struct {
	// ProviderEnv evaluates `gardenctl provider-env bash` before starting the tool.
	ProviderEnv bool
	// Gardenctl is the gardenctl binary (for ProviderEnv).
	Gardenctl string
	// Shell replaces $SHELL (default: Shell()).
	Shell string
}

// providerEnvScript evaluates the provider env in bash and then replaces itself
// with the tool. provider-env output uses bash syntax ($(< file)) and may run
// provider CLI logins, so it is evaluated rather than parsed.
const providerEnvScript = `script="$("$GTUI_GARDENCTL" provider-env bash)" || exit; eval "$script" || exit; unset script; exec "$@"`

// Command returns the exec.Cmd for tool in session s.
func Command(tool config.Tool, s *session.Session, o Options) (*exec.Cmd, error) {
	if len(tool.Command) == 0 {
		return nil, fmt.Errorf("tool %q has no command", tool.Name)
	}
	shell := o.Shell
	if shell == "" {
		shell = Shell()
	}
	argv := Expand(tool.Command, s.Target, shell)
	env := append([]string(nil), s.Env...)
	if o.ProviderEnv {
		gctl := o.Gardenctl
		if gctl == "" {
			gctl = "gardenctl"
		}
		env = append(env, "GTUI_GARDENCTL="+gctl)
		argv = append([]string{"bash", "-c", providerEnvScript, "gtui-provider-env"}, argv...)
	}
	cmd := exec.Command(argv[0], argv[1:]...) // #nosec G204 -- user-configured tool
	cmd.Env = env
	return cmd, nil
}

// Exec wraps an exec.Cmd as a tea.ExecCommand that prints a banner before the
// tool starts, so that the user knows where they are and how to get back.
type Exec struct {
	Cmd    *exec.Cmd
	Banner string
	// Stderr, if set, additionally receives a copy of the command's stderr.
	Stderr io.Writer
}

func (e *Exec) SetStdin(r io.Reader) {
	if e.Cmd.Stdin == nil {
		e.Cmd.Stdin = r
	}
}

func (e *Exec) SetStdout(w io.Writer) {
	if e.Cmd.Stdout == nil {
		e.Cmd.Stdout = w
	}
}

// SetStderr ignores w: Bubble Tea passes os.Stderr, which gtui redirects to a
// log file while the UI runs. Run uses the terminal (stdout) instead, so shells
// stay interactive and error messages stay visible.
func (e *Exec) SetStderr(io.Writer) {}

func (e *Exec) Run() error {
	if e.Cmd.Stderr == nil {
		e.Cmd.Stderr = e.Cmd.Stdout
		if e.Stderr != nil {
			e.Cmd.Stderr = io.MultiWriter(e.Cmd.Stdout, e.Stderr)
		}
	}
	if e.Banner != "" && e.Cmd.Stdout != nil {
		fmt.Fprintln(e.Cmd.Stdout, e.Banner)
	}
	return e.Cmd.Run()
}
