package launcher

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/session"
)

var tgt = session.Target{Garden: "g", Project: "p", Shoot: "s", Namespace: "garden-p"}

func TestExpand(t *testing.T) {
	got := Expand([]string{"$SHELL", "-c", "echo {garden}/{project}/{shoot} {namespace} ${SHELL}"}, tgt, "/bin/zsh")
	want := []string{"/bin/zsh", "-c", "echo g/p/s garden-p /bin/zsh"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestAvailable(t *testing.T) {
	defer func(f func(string) (string, error)) { LookPath = f }(LookPath)
	LookPath = func(bin string) (string, error) {
		if bin == "k9s" {
			return "/usr/bin/k9s", nil
		}
		return "", errors.New("not found")
	}
	if !Available(config.Tool{Command: []string{"k9s"}}) {
		t.Fatal("k9s should be available")
	}
	if Available(config.Tool{Command: []string{"sofka"}}) || Available(config.Tool{}) {
		t.Fatal("sofka should not be available")
	}
}

func TestCommand(t *testing.T) {
	s := &session.Session{Target: tgt, Env: []string{"GCTL_SESSION_ID=x", "KUBECONFIG=/kc"}}
	cmd, err := Command(config.Tool{Name: "k", Command: []string{"k9s", "-n", "{namespace}"}}, s, Options{Shell: "/bin/bash"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cmd.Args, []string{"k9s", "-n", "garden-p"}) || !reflect.DeepEqual(cmd.Env, s.Env) {
		t.Fatalf("cmd: %q env %q", cmd.Args, cmd.Env)
	}

	cmd, err = Command(config.Tool{Command: []string{"$SHELL"}}, s, Options{Shell: "/bin/zsh", ProviderEnv: true, Gardenctl: "/opt/gardenctl"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Args[0] != "bash" || cmd.Args[len(cmd.Args)-1] != "/bin/zsh" || cmd.Env[len(cmd.Env)-1] != "GTUI_GARDENCTL=/opt/gardenctl" {
		t.Fatalf("provider env cmd: %q env %q", cmd.Args, cmd.Env)
	}
	if &cmd.Env[0] == &s.Env[0] {
		t.Fatal("session env must not be aliased")
	}

	if _, err := Command(config.Tool{Name: "empty"}, s, Options{}); err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestProviderEnvScriptRuns(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := t.TempDir()
	fake := dir + "/gardenctl"
	script := "#!/bin/sh\necho \"export AWS_REGION='eu-1';\"\n"
	if err := writeExec(fake, script); err != nil {
		t.Fatal(err)
	}
	s := &session.Session{Target: tgt, Env: []string{"PATH=/usr/bin:/bin"}}
	cmd, err := Command(config.Tool{Command: []string{"sh", "-c", "echo $AWS_REGION {shoot}"}}, s, Options{ProviderEnv: true, Gardenctl: fake})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	e := &Exec{Cmd: cmd, Banner: "gtui banner"}
	e.SetStdout(&out)
	e.SetStderr(&out)
	if err := e.Run(); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if got := out.String(); got != "gtui banner\neu-1 s\n" {
		t.Fatalf("output %q", got)
	}

	// A failing provider-env must not start the tool.
	if err := writeExec(fake, "#!/bin/sh\necho 'Error: no credentials' >&2; exit 1\n"); err != nil {
		t.Fatal(err)
	}
	cmd, _ = Command(config.Tool{Command: []string{"sh", "-c", "echo started"}}, s, Options{ProviderEnv: true, Gardenctl: fake})
	out.Reset()
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err == nil || strings.Contains(out.String(), "started") {
		t.Fatalf("tool started despite provider-env failure: %v %q", err, out.String())
	}
}

// Bubble Tea passes os.Stderr (redirected to a log file by gtui); the tool must
// write its stderr to the terminal instead so shells stay interactive.
func TestExecSendsStderrToTerminal(t *testing.T) {
	term, err := os.CreateTemp(t.TempDir(), "tty") // like the real terminal, an *os.File
	if err != nil {
		t.Fatal(err)
	}
	var logFile, captured bytes.Buffer
	e := &Exec{Cmd: exec.Command("sh", "-c", "echo out; echo err >&2"), Stderr: &captured}
	e.SetStdout(term)
	e.SetStderr(&logFile)
	if err := e.Run(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(term.Name())
	if string(got) != "out\nerr\n" || logFile.Len() != 0 || captured.String() != "err\n" {
		t.Fatalf("terminal %q, log %q, captured %q", got, logFile.String(), captured.String())
	}
}

func TestWindowCommand(t *testing.T) {
	t.Setenv("GTUI_TEST_SAME", "1")
	cmd := exec.Command("k9s", "-n", "x")
	cmd.Env = []string{"GTUI_TEST_SAME=1", "KUBECONFIG=/kc"}
	w, err := WindowCommand(cmd, []string{"tmux", "new-window"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"tmux", "new-window", "env", "KUBECONFIG=/kc", "k9s", "-n", "x"}
	if !reflect.DeepEqual(w.Args, want) || !reflect.DeepEqual(w.Env, cmd.Env) {
		t.Fatalf("args %q env %q", w.Args, w.Env)
	}
	if w.Stdin != nil || w.Stdout != nil || w.SysProcAttr == nil || !w.SysProcAttr.Setsid {
		t.Fatal("window command must be detached")
	}
	if _, err := WindowCommand(cmd, nil); err == nil {
		t.Fatal("expected error without terminal")
	}
}

func TestDetectTerminal(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1/default,1,0")
	if got := DetectTerminal(); !reflect.DeepEqual(got, []string{"tmux", "new-window"}) {
		t.Fatalf("got %q", got)
	}
}
