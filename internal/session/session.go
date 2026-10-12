package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const (
	EnvSessionID  = "GCTL_SESSION_ID"
	EnvKubeconfig = "KUBECONFIG"
	EnvTarget     = "GTUI_TARGET"
)

// MinGardenctlVersion is the oldest gardenctl version gtui was written against
// (session directories under $TMPDIR/garden/sessions were introduced in v2.10.0).
const MinGardenctlVersion = "v2.10.0"

// ErrConfirmationRequired is returned when `gardenctl target` asks for a
// confirmation (access restrictions). The caller must run the target command
// interactively (see Manager.TargetCommand).
var ErrConfirmationRequired = errors.New("gardenctl requires confirmation")

var sidRegexp = regexp.MustCompile(`^[\w-]{1,128}$`)

// Runner executes gardenctl. It is an interface so that tests can mock gardenctl.
type Runner interface {
	// Run executes gardenctl with args and the given environment (non-interactive).
	Run(ctx context.Context, env []string, args ...string) (stdout, stderr string, err error)
	// Command returns an interactive command (stdio is attached by the caller).
	Command(env []string, args ...string) *exec.Cmd
}

// ExecRunner runs the gardenctl binary.
type ExecRunner struct{ Binary string }

func (r ExecRunner) bin() string {
	if r.Binary == "" {
		return "gardenctl"
	}
	return r.Binary
}

func (r ExecRunner) Run(ctx context.Context, env []string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, r.bin(), args...) // #nosec G204 -- configured binary
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func (r ExecRunner) Command(env []string, args ...string) *exec.Cmd {
	cmd := exec.Command(r.bin(), args...) // #nosec G204 -- configured binary
	cmd.Env = env
	return cmd
}

// Session is an isolated gardenctl session for one launched tool.
type Session struct {
	ID         string
	Target     Target
	Kubeconfig string
	// Env is the complete environment for the child process.
	Env []string
}

// Manager creates gardenctl sessions and removes them again.
type Manager struct {
	Runner Runner
	// TempDir is the base of gardenctl's temp dir (default os.TempDir()).
	TempDir string
	// BaseEnv is the environment sessions start from (default os.Environ()).
	BaseEnv []string
	// NewID generates session IDs (default random UUIDs).
	NewID func() string

	mu      sync.Mutex
	created []string
}

// NewManager returns a Manager that calls the given gardenctl binary.
func NewManager(gardenctl string) *Manager {
	return &Manager{Runner: ExecRunner{Binary: gardenctl}}
}

// New allocates a session for t (nothing is executed yet).
func (m *Manager) New(t Target) *Session {
	id := uuid.NewString()
	if m.NewID != nil {
		id = m.NewID()
	}
	m.mu.Lock()
	m.created = append(m.created, id)
	m.mu.Unlock()

	base := m.BaseEnv
	if base == nil {
		base = os.Environ()
	}
	env := make([]string, 0, len(base)+3)
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if k == EnvSessionID || k == EnvKubeconfig || k == EnvTarget {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, EnvSessionID+"="+id, EnvTarget+"="+t.String())
	return &Session{ID: id, Target: t, Env: env}
}

// Target runs `gardenctl target` non-interactively for the session.
// It returns ErrConfirmationRequired if gardenctl asks for confirmation.
func (m *Manager) Target(ctx context.Context, s *Session) error {
	stdout, stderr, err := m.Runner.Run(ctx, s.Env, s.Target.TargetArgs()...)
	if strings.Contains(stdout, "Do you want to continue?") {
		return ErrConfirmationRequired
	}
	if err != nil {
		return gardenctlError("target", stderr, err)
	}
	return nil
}

// TargetCommand returns `gardenctl target` as an interactive command, for
// targets that require a confirmation.
func (m *Manager) TargetCommand(s *Session) *exec.Cmd {
	return m.Runner.Command(s.Env, s.Target.TargetArgs()...)
}

// Env resolves KUBECONFIG via `gardenctl kubectl-env bash` and adds it to the session env.
func (m *Manager) Env(ctx context.Context, s *Session) error {
	stdout, stderr, err := m.Runner.Run(ctx, s.Env, "kubectl-env", "bash")
	if err != nil {
		return gardenctlError("kubectl-env", stderr, err)
	}
	kc, err := ParseKubeconfigExport(stdout)
	if err != nil {
		return err
	}
	s.Kubeconfig = kc
	s.Env = append(s.Env, EnvKubeconfig+"="+kc)
	return nil
}

// Prepare targets and resolves the environment in one go.
func (m *Manager) Prepare(ctx context.Context, t Target) (*Session, error) {
	s := m.New(t)
	if err := m.Target(ctx, s); err != nil {
		return s, err
	}
	return s, m.Env(ctx, s)
}

// Version returns the gardenctl version (e.g. "v2.16.0").
func (m *Manager) Version(ctx context.Context) (string, error) {
	stdout, stderr, err := m.Runner.Run(ctx, os.Environ(), "version", "-o", "json")
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) || errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("gardenctl not found: %w", err)
		}
		return "", gardenctlError("version", stderr, err)
	}
	var info struct {
		GitVersion string `json:"gitVersion"`
	}
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		return "", fmt.Errorf("parse gardenctl version: %w", err)
	}
	return info.GitVersion, nil
}

// Cleanup removes the gardenctl session directories of all sessions created
// by this manager (and only those).
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	ids := m.created
	m.created = nil
	m.mu.Unlock()

	base := m.TempDir
	if base == "" {
		base = os.TempDir()
	}
	var errs []error
	for _, id := range ids {
		if !sidRegexp.MatchString(id) {
			continue
		}
		for _, dir := range []string{
			filepath.Join(base, "garden", "sessions", id),
			filepath.Join(base, "garden", id), // layout before gardenctl v2.10.0
		} {
			if err := os.RemoveAll(dir); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func gardenctlError(cmd, stderr string, err error) error {
	msg := strings.TrimSpace(stderr)
	if i := strings.LastIndex(msg, "\n"); i >= 0 {
		msg = strings.TrimSpace(msg[i+1:]) // the last line carries the actual error
	}
	msg = strings.TrimPrefix(msg, "Error: ")
	if msg == "" {
		return fmt.Errorf("gardenctl %s: %w", cmd, err)
	}
	return fmt.Errorf("gardenctl %s: %s", cmd, msg)
}

// CompareVersions compares two "vX.Y.Z[-suffix]" versions and returns -1, 0 or 1.
// Unparsable parts compare as 0.
func CompareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < 3; i++ {
		switch {
		case pa[i] < pb[i]:
			return -1
		case pa[i] > pb[i]:
			return 1
		}
	}
	return 0
}

func versionParts(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(v, "v")
	v, _, _ = strings.Cut(v, "-")
	v, _, _ = strings.Cut(v, "+")
	for i, p := range strings.SplitN(v, ".", 3) {
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		out[i] = n
	}
	return out
}
