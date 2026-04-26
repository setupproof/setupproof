package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/setupproof/setupproof/internal/planning"
)

func TestDockerRunArgsUseSafetyControlsAndWorkspaceScopedMount(t *testing.T) {
	args := dockerRunArgs(dockerRunSpec{
		Name:          "setupproof-test",
		Image:         "ubuntu@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Workspace:     "/tmp/workspace",
		User:          "123:456",
		NetworkPolicy: "disabled",
		Env: []string{
			"HOME=" + containerHome,
			"TMPDIR=" + containerTmp,
			"XDG_CACHE_HOME=" + containerCache,
			"CI=true",
			"SETUPPROOF=1",
		},
	})
	joined := strings.Join(args, "\x00")
	for _, want := range []string{
		"run", "--detach",
		"--name\x00setupproof-test",
		"--workdir\x00" + containerWorkspaceRoot,
		"--mount\x00type=bind,src=/tmp/workspace,dst=" + containerWorkspaceRoot,
		"--user\x00123:456",
		"--read-only",
		"--tmpfs\x00/tmp",
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges",
		"--network\x00none",
		"--env\x00HOME=" + containerHome,
		"--env\x00TMPDIR=" + containerTmp,
		"--env\x00XDG_CACHE_HOME=" + containerCache,
		"ubuntu@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("docker run args missing %q:\n%#v", want, args)
		}
	}
	for _, forbidden := range []string{"--privileged", "-t", "--tty"} {
		if hasToken(args, forbidden) {
			t.Fatalf("docker run args contain forbidden value %q:\n%#v", forbidden, args)
		}
	}
	for _, forbidden := range []string{"/var/run/docker.sock", os.Getenv("HOME")} {
		if forbidden != "" && strings.Contains(joined, forbidden) {
			t.Fatalf("docker run args contain forbidden value %q:\n%#v", forbidden, args)
		}
	}
}

func TestDockerExecArgsUseNoTTYAndExplicitState(t *testing.T) {
	args := dockerExecArgs(dockerExecSpec{
		Name:   "setupproof-test",
		CWD:    "/workspace/subdir",
		Env:    []string{"SETUPPROOF=1", "HOME=" + containerHome},
		Shell:  "bash",
		Script: "/workspace/.setupproof/control/block.sh",
	})
	joined := strings.Join(args, "\x00")
	for _, want := range []string{
		"exec",
		"--workdir\x00/workspace/subdir",
		"--env\x00HOME=" + containerHome,
		"--env\x00SETUPPROOF=1",
		"setupproof-test\x00bash\x00/workspace/.setupproof/control/block.sh",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("docker exec args missing %q:\n%#v", want, args)
		}
	}
	for _, forbidden := range []string{"-t", "--tty", "-i", "--interactive"} {
		if hasToken(args, forbidden) {
			t.Fatalf("docker exec args contain forbidden value %q:\n%#v", forbidden, args)
		}
	}
}

func TestDockerBaselineEnvironmentKeepsHomeAndCachesInWorkspace(t *testing.T) {
	t.Setenv("PATH", "host-path-should-not-pass")
	t.Setenv("SETUPPROOF_DOCKER_ALLOWED_TEST", "allowed")
	env, _, err := baselineDockerEnv(planning.Env{Allow: []string{"SETUPPROOF_DOCKER_ALLOWED_TEST"}}, ioDiscard())
	if err != nil {
		t.Fatal(err)
	}
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{
		"\nHOME=" + containerHome + "\n",
		"\nTMPDIR=" + containerTmp + "\n",
		"\nXDG_CACHE_HOME=" + containerCache + "\n",
		"\nnpm_config_cache=" + containerCache + "/npm\n",
		"\nPIP_CACHE_DIR=" + containerCache + "/pip\n",
		"\nCARGO_HOME=" + containerCache + "/cargo\n",
		"\nGOCACHE=" + containerCache + "/go-build\n",
		"\nSETUPPROOF_DOCKER_ALLOWED_TEST=allowed\n",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("docker env missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "\nPATH=") {
		t.Fatalf("docker env inherited host PATH:\n%s", joined)
	}
}

func TestDockerCommandErrorClassification(t *testing.T) {
	cases := []struct {
		name string
		in   dockerCommandResult
		want string
	}{
		{name: "missing docker", in: dockerCommandResult{err: exec.ErrNotFound}, want: "docker_unavailable"},
		{name: "daemon", in: dockerCommandResult{exitCode: 1, stderr: "Cannot connect to the Docker daemon"}, want: "docker_unavailable"},
		{name: "pull", in: dockerCommandResult{exitCode: 125, stderr: "manifest unknown"}, want: "image_pull_failed"},
		{name: "start", in: dockerCommandResult{exitCode: 125, stderr: "docker run failed"}, want: "process_start_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyDockerCommandError(tc.in); got != tc.want {
				t.Fatalf("classification = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDockerRunnerClassifiesSetupFailureAsInfrastructure(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "```sh setupproof runner=docker image=missing:latest id=docker\ntrue\n```\n")
	gitAdd(t, dir, "README.md")
	installFakeDocker(t, `case "$1" in
run)
  printf 'manifest unknown\n' >&2
  exit 125
  ;;
*)
  exit 0
  ;;
esac
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "docker runner error: image_pull_failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestDockerRunnerExecutesThroughDockerLifecycleAndCleansContainer(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "```sh setupproof runner=docker id=docker\ntrue\n```\n")
	gitAdd(t, dir, "README.md")
	logPath := filepath.Join(t.TempDir(), "docker.log")
	stateDir := t.TempDir()
	installFakeDocker(t, `printf '%s\n' "$*" >> "$FAKE_DOCKER_LOG"
case "$1" in
run)
  name=
  root=
  shift
  while [ "$#" -gt 0 ]; do
    case "$1" in
    --name)
      name=$2
      shift 2
      ;;
    --mount)
      root=${2#type=bind,src=}
      root=${root%%,dst=*}
      shift 2
      ;;
    --workdir|--network|--env|--user|--tmpfs)
      shift 2
      ;;
    --detach|--read-only|--cap-drop=*|--security-opt=*)
      shift
      ;;
    *)
      shift
      ;;
    esac
  done
  printf '%s' "$root" > "$FAKE_DOCKER_STATE/$name.root"
  printf 'container-id\n'
  ;;
image)
  printf '["ubuntu:24.04@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]\n'
  ;;
exec)
  shift
  workdir=/workspace
  while [ "$#" -gt 0 ]; do
    case "$1" in
    --workdir)
      workdir=$2
      shift 2
      ;;
    --env)
      shift 2
      ;;
    *)
      break
      ;;
    esac
  done
  name=$1
  script=$3
  root=$(cat "$FAKE_DOCKER_STATE/$name.root")
  local_script=$root${script#/workspace}
  state_cwd=$(sed -n "s/^__setupproof_[0-9a-f][0-9a-f]*_state_cwd='\(.*\)'$/\1/p" "$local_script")
  state_env=$(sed -n "s/^__setupproof_[0-9a-f][0-9a-f]*_state_env='\(.*\)'$/\1/p" "$local_script")
  local_cwd=$root${state_cwd#/workspace}
  local_env=$root${state_env#/workspace}
  mkdir -p "$(dirname "$local_cwd")" "$(dirname "$local_env")"
  printf '%s\n' "$workdir" > "$local_cwd"
  printf '%s\0' HOME=/workspace/.setupproof/home SETUPPROOF=1 CI=true > "$local_env"
  ;;
rm)
  exit 0
  ;;
*)
  exit 0
  ;;
esac
`)
	t.Setenv("FAKE_DOCKER_LOG", logPath)
	t.Setenv("FAKE_DOCKER_STATE", stateDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "runner=docker") || !strings.Contains(stdout.String(), "result=passed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logBytes)
	for _, want := range []string{"run --detach", "exec --workdir /workspace", "rm -f setupproof-"} {
		if !strings.Contains(log, want) {
			t.Fatalf("docker lifecycle log missing %q:\n%s", want, log)
		}
	}
	if !strings.Contains(stderr.String(), "docker image digest: ubuntu:24.04@sha256:") {
		t.Fatalf("stderr did not record resolved digest:\n%s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".setupproof")); !os.IsNotExist(err) {
		t.Fatalf("docker runner mutated live repository state: %v", err)
	}
}

func TestDockerRunnerCleansContainerAfterCommandFailure(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "```sh setupproof runner=docker id=docker\nfalse\n```\n")
	gitAdd(t, dir, "README.md")
	logPath := filepath.Join(t.TempDir(), "docker.log")
	installFakeDocker(t, `printf '%s\n' "$*" >> "$FAKE_DOCKER_LOG"
case "$1" in
run)
  printf 'container-id\n'
  ;;
exec)
  exit 1
  ;;
rm)
  exit 0
  ;;
*)
  exit 0
  ;;
esac
`)
	t.Setenv("FAKE_DOCKER_LOG", logPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "result=failed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logBytes), "rm -f setupproof-") {
		t.Fatalf("docker rm was not invoked after failure:\n%s", string(logBytes))
	}
}

func TestDockerExecTimeoutRemovesContainer(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "docker.log")
	installFakeDocker(t, `printf '%s\n' "$*" >> "$FAKE_DOCKER_LOG"
case "$1" in
exec)
  sleep 10
  ;;
rm)
  exit 0
  ;;
*)
  exit 0
  ;;
esac
`)
	t.Setenv("FAKE_DOCKER_LOG", logPath)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result := dockerExec(ctx, "setupproof-timeout", containerWorkspaceRoot, []string{"SETUPPROOF=1"}, "sh", "/workspace/block.sh", ioDiscard(), nil)
	if !result.timedOut {
		t.Fatalf("expected timeout, got %#v", result)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logBytes), "rm -f setupproof-timeout") {
		t.Fatalf("docker rm was not invoked after timeout:\n%s", string(logBytes))
	}
}

func TestValidateDockerPlanRejectsMixedSharedImages(t *testing.T) {
	plan := planning.Plan{Blocks: []planning.Block{
		{File: "README.md", QualifiedID: "README.md#one", Options: planning.Options{Runner: "docker", DockerImage: "ubuntu:24.04"}},
		{File: "README.md", QualifiedID: "README.md#two", Options: planning.Options{Runner: "docker", DockerImage: "alpine:3.20"}},
	}}
	if err := validateDockerPlan(plan); err == nil {
		t.Fatal("expected mixed image validation error")
	}
}

func TestDockerImageRecorderRecordsEachImageOnce(t *testing.T) {
	installFakeDocker(t, `case "$1" in
image)
  printf '["ubuntu:24.04@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]\n'
  ;;
*)
  exit 0
  ;;
esac
`)
	var stderr bytes.Buffer
	recorder := newDockerImageRecorder()
	recorder.record(context.Background(), "ubuntu:24.04", &stderr)
	recorder.record(context.Background(), "ubuntu:24.04", &stderr)

	output := stderr.String()
	if strings.Count(output, "warning: docker image ubuntu:24.04") != 1 {
		t.Fatalf("stderr = %q", output)
	}
	if strings.Count(output, "docker image digest:") != 1 {
		t.Fatalf("stderr = %q", output)
	}
}

func TestDockerContainerNameUsesRandomSuffix(t *testing.T) {
	name := dockerContainerName()
	if !strings.HasPrefix(name, "setupproof-") {
		t.Fatalf("container name = %q", name)
	}
	parts := strings.Split(name, "-")
	if len(parts) != 4 {
		t.Fatalf("container name should include pid, timestamp, and random suffix: %q", name)
	}
	if len(parts[3]) != 12 {
		t.Fatalf("random suffix length = %d in %q", len(parts[3]), name)
	}
}

func TestDockerWorkspaceUsesUserCacheForDaemonVisibleMounts(t *testing.T) {
	parent := t.TempDir()
	t.Setenv("HOME", parent)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(parent, ".cache"))
	root := filepath.Join(parent, "repo")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	wantParent := filepath.Join(cacheDir, "setupproof", "docker-workspaces")
	ws, err := createDockerWorkspace(workspaceSource{root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = ws.cleanup(false)
	}()
	if got := filepath.Dir(ws.tempRoot); got != wantParent {
		t.Fatalf("docker workspace parent = %q, want %q", got, wantParent)
	}
}

func TestDockerRunnerWritesHostOwnedFilesIntegration(t *testing.T) {
	if os.Getenv("SETUPPROOF_INTEGRATION_DOCKER") != "1" {
		t.Skip("set SETUPPROOF_INTEGRATION_DOCKER=1 to run real Docker integration tests")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker is not available")
	}

	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "```sh setupproof runner=docker image=alpine:3.20 id=docker-user\n"+
		"id -u > uid.txt\n"+
		"id -g > gid.txt\n"+
		"touch generated.txt\n"+
		"```\n")
	gitAdd(t, dir, "README.md")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{KeepWorkspace: true}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	workspacePath := keptWorkspacePath(t, stderr.String())
	defer func() {
		_ = os.RemoveAll(filepath.Dir(workspacePath))
	}()

	uid := strings.TrimSpace(readFile(t, filepath.Join(workspacePath, "uid.txt")))
	gid := strings.TrimSpace(readFile(t, filepath.Join(workspacePath, "gid.txt")))
	if uid != fmt.Sprint(os.Getuid()) || gid != fmt.Sprint(os.Getgid()) {
		t.Fatalf("container wrote as %s:%s, want %d:%d", uid, gid, os.Getuid(), os.Getgid())
	}
}

func installFakeDocker(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func ioDiscard() *bytes.Buffer {
	return &bytes.Buffer{}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func hasToken(values []string, token string) bool {
	for _, value := range values {
		if value == token {
			return true
		}
	}
	return false
}
