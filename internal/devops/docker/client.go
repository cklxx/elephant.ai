package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Client provides a type-safe interface for Docker operations.
type Client interface {
	ContainerExists(ctx context.Context, name string) (bool, error)
	ContainerRunning(ctx context.Context, name string) (bool, error)
	ContainerCreate(ctx context.Context, opts CreateOpts) error
	ContainerStart(ctx context.Context, name string) error
	ContainerStop(ctx context.Context, name string, timeout time.Duration) error
	ContainerRemove(ctx context.Context, name string) error
	ContainerInspect(ctx context.Context, name string) (*ContainerInfo, error)
	Exec(ctx context.Context, container string, cmd []string, opts ExecOpts) (string, error)
	CopyTo(ctx context.Context, container string, src, dst string) error
	PortMapping(ctx context.Context, container string, port int) (string, error)
	ImagePull(ctx context.Context, image string) error
}

// CreateOpts defines options for creating a container.
type CreateOpts struct {
	Name       string
	Image      string
	Ports      map[int]int       // container:host
	Volumes    map[string]string // host:container
	Env        map[string]string
	ExtraHosts []string
}

// ExecOpts defines options for exec in a container.
type ExecOpts struct {
	Env     map[string]string
	WorkDir string
	Stdin   io.Reader
	Detach  bool
}

// ContainerInfo holds inspect results.
type ContainerInfo struct {
	Name    string
	Running bool
	Image   string
	Mounts  []MountInfo
	Ports   []PortInfo
}

// MountInfo describes a container mount.
type MountInfo struct {
	Source      string
	Destination string
}

// PortInfo describes a port mapping.
type PortInfo struct {
	ContainerPort int
	HostPort      int
	Protocol      string
}

// CLIClient implements Client by shelling out to the docker CLI.
type CLIClient struct {
	dockerBin string
}

// NewCLIClient creates a new CLI-based Docker client.
func NewCLIClient() *CLIClient {
	bin := "docker"
	if p, err := exec.LookPath("docker"); err == nil {
		bin = p
	}
	return &CLIClient{dockerBin: bin}
}

func (c *CLIClient) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.dockerBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *CLIClient) ContainerExists(ctx context.Context, name string) (bool, error) {
	out, err := c.run(ctx, "ps", "-a", "--format", "{{.Names}}")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

func (c *CLIClient) ContainerRunning(ctx context.Context, name string) (bool, error) {
	out, err := c.run(ctx, "ps", "--format", "{{.Names}}")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

func (c *CLIClient) ContainerCreate(ctx context.Context, opts CreateOpts) error {
	args := []string{"run", "-d", "--name", opts.Name}

	for _, host := range opts.ExtraHosts {
		args = append(args, "--add-host", host)
	}
	for k, v := range opts.Env {
		args = append(args, "-e", k+"="+v)
	}
	for containerPort, hostPort := range opts.Ports {
		args = append(args, "-p", fmt.Sprintf("%d:%d", hostPort, containerPort))
	}
	for hostPath, containerPath := range opts.Volumes {
		args = append(args, "-v", hostPath+":"+containerPath)
	}
	args = append(args, opts.Image)

	_, err := c.run(ctx, args...)
	return err
}

func (c *CLIClient) ContainerStart(ctx context.Context, name string) error {
	_, err := c.run(ctx, "start", name)
	return err
}

func (c *CLIClient) ContainerStop(ctx context.Context, name string, timeout time.Duration) error {
	args := []string{"stop"}
	if timeout > 0 {
		args = append(args, "-t", strconv.Itoa(int(timeout.Seconds())))
	}
	args = append(args, name)
	_, err := c.run(ctx, args...)
	return err
}

func (c *CLIClient) ContainerRemove(ctx context.Context, name string) error {
	_, err := c.run(ctx, "rm", name)
	return err
}

func (c *CLIClient) ContainerInspect(ctx context.Context, name string) (*ContainerInfo, error) {
	out, err := c.run(ctx, "inspect", name)
	if err != nil {
		return nil, err
	}

	var inspections []dockerInspection
	if err := json.Unmarshal([]byte(out), &inspections); err != nil {
		return nil, fmt.Errorf("parse inspect output: %w", err)
	}
	if len(inspections) == 0 {
		return nil, fmt.Errorf("no inspection data for %s", name)
	}

	insp := inspections[0]
	info := &ContainerInfo{
		Name:    name,
		Running: insp.State.Running,
		Image:   insp.Config.Image,
	}

	for _, m := range insp.Mounts {
		info.Mounts = append(info.Mounts, MountInfo{
			Source:      m.Source,
			Destination: m.Destination,
		})
	}

	for containerPort, bindings := range insp.NetworkSettings.Ports {
		parts := strings.SplitN(string(containerPort), "/", 2)
		port, _ := strconv.Atoi(parts[0])
		proto := "tcp"
		if len(parts) > 1 {
			proto = parts[1]
		}
		for _, b := range bindings {
			hp, _ := strconv.Atoi(b.HostPort)
			info.Ports = append(info.Ports, PortInfo{
				ContainerPort: port,
				HostPort:      hp,
				Protocol:      proto,
			})
		}
	}

	return info, nil
}

func (c *CLIClient) Exec(ctx context.Context, container string, cmd []string, opts ExecOpts) (string, error) {
	args := []string{"exec"}
	if opts.Detach {
		args = append(args, "-d")
	}
	if opts.WorkDir != "" {
		args = append(args, "-w", opts.WorkDir)
	}
	for k, v := range opts.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, container)
	args = append(args, cmd...)

	execCmd := exec.CommandContext(ctx, c.dockerBin, args...)
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	if opts.Stdin != nil {
		execCmd.Stdin = opts.Stdin
	}

	if err := execCmd.Run(); err != nil {
		return "", fmt.Errorf("docker exec in %s: %s: %w", container, strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *CLIClient) CopyTo(ctx context.Context, container string, src, dst string) error {
	_, err := c.run(ctx, "cp", src, container+":"+dst)
	return err
}

func (c *CLIClient) PortMapping(ctx context.Context, container string, port int) (string, error) {
	return c.run(ctx, "port", container, fmt.Sprintf("%d/tcp", port))
}

func (c *CLIClient) ImagePull(ctx context.Context, image string) error {
	_, err := c.run(ctx, "pull", image)
	return err
}

type dockerInspection struct {
	State struct {
		Running bool `json:"Running"`
	} `json:"State"`
	Config struct {
		Image string `json:"Image"`
	} `json:"Config"`
	Mounts []struct {
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
	NetworkSettings struct {
		Ports map[dockerPort][]dockerPortBinding `json:"Ports"`
	} `json:"NetworkSettings"`
}

type dockerPort string

type dockerPortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}
