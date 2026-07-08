package sandbox

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type Status string

const (
	StatusCreated Status = "created"
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusCrashed Status = "crashed"
)

type Sandbox struct {
	ID         string
	AppID      string
	Port       int
	BinaryPath string
	Status     Status
	PID        int
	Cmd        *exec.Cmd
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Manager struct {
	basePath string
	mu       sync.RWMutex
	items    map[string]*Sandbox
}

func NewManager(basePath string) *Manager {
	return &Manager{basePath: basePath, items: map[string]*Sandbox{}}
}

func (m *Manager) BuildBwrapCommand(appID string, binaryPath string, env []string) *exec.Cmd {
	workDir := filepath.Join(m.basePath, appID)
	args := []string{
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/lib64", "/lib64",
		"--ro-bind", "/bin", "/bin",
		"--tmpfs", "/tmp",
		"--proc", "/proc",
		"--dev", "/dev",
		"--bind", workDir, "/app",
		"--unshare-user",
		"--unshare-ipc",
		"--unshare-pid",
		"--hostname", fmt.Sprintf("sandbox-%s", appID),
		binaryPath,
	}
	cmd := exec.Command("bwrap", args...)
	cmd.Env = append(cmd.Env, env...)
	cmd.Dir = workDir
	return cmd
}
