package runner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/setupproof/setupproof/internal/planning"
	"github.com/setupproof/setupproof/internal/shellquote"
)

func TestLocalRunnerCopiesTrackedWorkingTreeState(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "```sh setupproof id=copy\n"+
		"grep -qx staged staged.txt\n"+
		"grep -qx unstaged unstaged.txt\n"+
		"```\n")
	writeFile(t, dir, "staged.txt", "staged\n")
	writeFile(t, dir, "unstaged.txt", "indexed\n")
	gitAdd(t, dir, "README.md", "staged.txt", "unstaged.txt")
	writeFile(t, dir, "unstaged.txt", "unstaged\n")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "README.md#copy") || !strings.Contains(stdout, "result=passed") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestLocalRunnerRewritesAbsoluteSymlinkTargetsIntoWorkspace(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "```sh setupproof id=symlink\n"+
		"printf 'mutated\\n' > abs-link.txt\n"+
		"grep -qx mutated abs-link.txt\n"+
		"```\n")
	writeFile(t, dir, "target.txt", "original\n")
	if err := os.Symlink(filepath.Join(dir, "target.txt"), filepath.Join(dir, "abs-link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gitAdd(t, dir, "README.md", "target.txt", "abs-link.txt")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	live, err := os.ReadFile(filepath.Join(dir, "target.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(live) != "original\n" {
		t.Fatalf("live repository target was mutated through copied symlink: %q", string(live))
	}
}

func TestLocalRunnerExcludesUntrackedAndIgnoredFilesByDefault(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, ".gitignore", "ignored.txt\n")
	writeFile(t, dir, "README.md", "```sh setupproof id=copy\n"+
		"test ! -e untracked.txt\n"+
		"test ! -e ignored.txt\n"+
		"```\n")
	writeFile(t, dir, "untracked.txt", "debug\n")
	writeFile(t, dir, "ignored.txt", "ignored\n")
	gitAdd(t, dir, ".gitignore", "README.md")

	code, _, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
}

func TestLocalRunnerIncludeUntrackedCopiesNonIgnoredFiles(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, ".gitignore", "ignored.txt\n")
	writeFile(t, dir, "README.md", "```sh setupproof id=copy\n"+
		"test -f untracked.txt\n"+
		"test ! -e ignored.txt\n"+
		"```\n")
	writeFile(t, dir, "untracked.txt", "debug\n")
	writeFile(t, dir, "ignored.txt", "ignored\n")
	gitAdd(t, dir, ".gitignore", "README.md")

	req := planning.Request{CWD: dir, Positional: []string{"README.md"}, IncludeUntracked: true}
	code, _, stderr := runLocal(t, dir, req, Options{})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "--include-untracked copied 1 untracked non-ignored file") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestLocalRunnerSharedStateAndFailureState(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "```sh setupproof id=one\n"+
		"mkdir work\n"+
		"cd work\n"+
		"export SETUPPROOF_STATE=good\n"+
		"echo ok > shared.txt\n"+
		"```\n\n"+
		"```sh setupproof id=fail\n"+
		"cd ..\n"+
		"export SETUPPROOF_STATE=bad\n"+
		"touch failed-mutation\n"+
		"false\n"+
		"```\n\n"+
		"```sh setupproof id=after\n"+
		"test \"$(basename \"$PWD\")\" = work\n"+
		"test \"$SETUPPROOF_STATE\" = good\n"+
		"test -f shared.txt\n"+
		"test -f ../failed-mutation\n"+
		"```\n")
	gitAdd(t, dir, "README.md")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 1 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for _, want := range []string{"README.md#one", "README.md#fail", "README.md#after"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if !blockLineContains(stdout, "README.md#after", "result=passed") {
		t.Fatalf("final block did not pass with preserved prior state:\n%s", stdout)
	}
}

func TestLocalRunnerDoesNotShareStateAcrossTargetFiles(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "one.md", "```sh setupproof id=one\n"+
		"touch shared.txt\n"+
		"export SETUPPROOF_FILE_STATE=one\n"+
		"```\n")
	writeFile(t, dir, "two.md", "```sh setupproof id=two\n"+
		"test ! -e shared.txt\n"+
		"test -z \"${SETUPPROOF_FILE_STATE:-}\"\n"+
		"```\n")
	gitAdd(t, dir, "one.md", "two.md")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"one.md", "two.md"}}, Options{})
	if code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !blockLineContains(stdout, "one.md#one", "result=passed") || !blockLineContains(stdout, "two.md#two", "result=passed") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestLocalRunnerIsolatedBlockUsesFreshWorkspaceAndDoesNotPersistFailureState(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "```sh setupproof id=shared\n"+
		"touch shared.txt\n"+
		"export SETUPPROOF_STATE=shared\n"+
		"```\n\n"+
		"```sh setupproof id=isolated isolated=true\n"+
		"test ! -e shared.txt\n"+
		"test -z \"${SETUPPROOF_STATE:-}\"\n"+
		"touch isolated.txt\n"+
		"export SETUPPROOF_STATE=isolated\n"+
		"false\n"+
		"```\n\n"+
		"```sh setupproof id=after\n"+
		"test -e shared.txt\n"+
		"test ! -e isolated.txt\n"+
		"test \"$SETUPPROOF_STATE\" = shared\n"+
		"```\n")
	gitAdd(t, dir, "README.md")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 1 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !blockLineContains(stdout, "README.md#after", "result=passed") {
		t.Fatalf("isolated block leaked state:\n%s", stdout)
	}
}

func TestLocalRunnerFailFastSkipsRemainingBlocksInFile(t *testing.T) {
	dir := gitRepo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	writeFile(t, dir, "README.md", "```sh setupproof id=fail\n"+
		"false\n"+
		"```\n\n"+
		"```sh setupproof id=skip\n"+
		"touch "+shellquote.Quote(marker)+"\n"+
		"```\n")
	gitAdd(t, dir, "README.md")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{FailFast: true})
	if code != 1 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "README.md#skip") || !strings.Contains(stdout, "result=skipped reason=fail-fast") {
		t.Fatalf("stdout = %q", stdout)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("fail-fast skipped block executed: %v", err)
	}
}

func TestLocalRunnerTimeoutDoesNotPersistStateAndKillsProcessTree(t *testing.T) {
	dir := gitRepo(t)
	leak := filepath.Join(t.TempDir(), "leaked")
	writeFile(t, dir, "README.md", "```sh setupproof id=one\n"+
		"export SETUPPROOF_STATE=good\n"+
		"```\n\n"+
		"```sh setupproof id=timeout timeout=1s\n"+
		"export SETUPPROOF_STATE=bad\n"+
		"(sleep 2; touch "+shellquote.Quote(leak)+") &\n"+
		"sleep 10\n"+
		"```\n\n"+
		"```sh setupproof id=after\n"+
		"test \"$SETUPPROOF_STATE\" = good\n"+
		"```\n")
	gitAdd(t, dir, "README.md")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 1 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "README.md#timeout") || !strings.Contains(stdout, "result=timeout") {
		t.Fatalf("stdout = %q", stdout)
	}
	if !blockLineContains(stdout, "README.md#after", "result=passed") {
		t.Fatalf("state from timed-out block persisted or later block did not run:\n%s", stdout)
	}
	time.Sleep(3 * time.Second)
	if _, err := os.Stat(leak); !os.IsNotExist(err) {
		t.Fatalf("timed-out process tree was not cleaned up: %v", err)
	}
}

func TestLocalRunnerClosesStdinAndAllocatesNoTTY(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "```sh setupproof id=stdio\n"+
		"if [ -t 0 ]; then exit 1; fi\n"+
		"if [ -t 1 ]; then exit 1; fi\n"+
		"cat > stdin.txt\n"+
		"test ! -s stdin.txt\n"+
		"```\n")
	gitAdd(t, dir, "README.md")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

func TestLocalRunnerUsesSanitizedBaselineEnvironmentAndAllowlist(t *testing.T) {
	dir := gitRepo(t)
	t.Setenv("SETUPPROOF_ALLOWED_ENV_TEST", "allowed")
	t.Setenv("SETUPPROOF_BLOCKED_ENV_TEST", "blocked")
	writeFile(t, dir, "setupproof.yml", "version: 1\n"+
		"env:\n"+
		"  allow:\n"+
		"    - SETUPPROOF_ALLOWED_ENV_TEST\n")
	writeFile(t, dir, "README.md", "```sh setupproof id=env\n"+
		"test \"$CI\" = true\n"+
		"test \"$SETUPPROOF\" = 1\n"+
		"test -n \"$PATH\"\n"+
		"test -d \"$HOME\"\n"+
		"test -d \"$TMPDIR\"\n"+
		"test \"$SETUPPROOF_ALLOWED_ENV_TEST\" = allowed\n"+
		"test -z \"${SETUPPROOF_BLOCKED_ENV_TEST:-}\"\n"+
		"```\n")
	gitAdd(t, dir, "README.md", "setupproof.yml")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

func TestLocalRunnerStrictShellModes(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available")
	}
	dir := gitRepo(t)
	shMarker := filepath.Join(t.TempDir(), "sh-ran")
	bashMarker := filepath.Join(t.TempDir(), "bash-ran")
	writeFile(t, dir, "README.md", "```sh setupproof id=sh-strict\n"+
		"false\n"+
		"touch "+shellquote.Quote(shMarker)+"\n"+
		"```\n\n"+
		"```bash setupproof id=bash-pipefail\n"+
		"false | true\n"+
		"touch "+shellquote.Quote(bashMarker)+"\n"+
		"```\n")
	gitAdd(t, dir, "README.md")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 1 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for _, marker := range []string{shMarker, bashMarker} {
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("strict mode allowed marker %s: %v", marker, err)
		}
	}
}

func TestLocalRunnerClassifiesInteractiveCommandsWithoutExecuting(t *testing.T) {
	dir := gitRepo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	writeFile(t, dir, "README.md", "```sh setupproof id=interactive\n"+
		"read answer\n"+
		"touch "+shellquote.Quote(marker)+"\n"+
		"```\n")
	gitAdd(t, dir, "README.md")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 1 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "reason=interactive-command") || !strings.Contains(stderr, "common interactive command") {
		t.Fatalf("stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("interactive command block executed: %v", err)
	}
}

func TestInteractiveClassifierJoinsShellContinuations(t *testing.T) {
	got, ok := classifyInteractive("re\\\nad answer\n")
	if !ok || got != "read" {
		t.Fatalf("classifyInteractive continuation = %q/%t", got, ok)
	}
}

func TestLocalRunnerCleansWorkspaceByDefaultAndKeepsWhenRequested(t *testing.T) {
	tempParent := t.TempDir()
	t.Setenv("TMPDIR", tempParent)
	defer func() {
		_ = makeTreeWritable(tempParent)
	}()

	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "```sh setupproof id=keep\n"+
		"touch kept.txt\n"+
		"```\n")
	gitAdd(t, dir, "README.md")

	code, _, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	assertNoWorkspaces(t, tempParent)

	writeFile(t, dir, "README.md", "```sh setupproof id=fail\nfalse\n```\n")
	gitAdd(t, dir, "README.md")
	code, _, stderr = runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 1 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	assertNoWorkspaces(t, tempParent)

	writeFile(t, dir, "README.md", "```sh setupproof id=timeout timeout=1s\nsleep 10\n```\n")
	gitAdd(t, dir, "README.md")
	code, _, stderr = runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 1 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	assertNoWorkspaces(t, tempParent)

	writeFile(t, dir, "README.md", "```sh setupproof id=readonly-cache\n"+
		"mkdir -p \"$HOME/go/pkg/mod/example.com/pkg@v1.0.0\"\n"+
		"printf 'package pkg\\n' > \"$HOME/go/pkg/mod/example.com/pkg@v1.0.0/pkg.go\"\n"+
		"chmod -R a-w \"$HOME/go/pkg/mod/example.com/pkg@v1.0.0\"\n"+
		"```\n")
	gitAdd(t, dir, "README.md")
	code, _, stderr = runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	assertNoWorkspaces(t, tempParent)

	writeFile(t, dir, "README.md", "```sh setupproof id=keep\n"+
		"touch kept.txt\n"+
		"```\n")
	gitAdd(t, dir, "README.md")
	code, _, stderr = runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{KeepWorkspace: true})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	path := keptWorkspacePath(t, stderr)
	if _, err := os.Stat(filepath.Join(path, "kept.txt")); err != nil {
		t.Fatalf("kept workspace path missing command output: %v\nstderr=%s", err, stderr)
	}
	if !strings.Contains(stderr, "unredacted environment captures from configured env.pass entries") {
		t.Fatalf("keep-workspace warning did not mention env captures:\n%s", stderr)
	}
	if err := os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
}

func TestSignalRunnerErrorDistinguishesCommonSignals(t *testing.T) {
	if got := signalRunnerError(os.Interrupt); got != "signal_interrupt" {
		t.Fatalf("interrupt reason = %q", got)
	}
	if got := signalRunnerError(syscall.SIGTERM); got != "signal_terminate" {
		t.Fatalf("terminate reason = %q", got)
	}
}

func TestCleanupFailurePreservesExistingExitCode(t *testing.T) {
	code, runnerError := codeWithCleanupFailure(1, "")
	if code != 1 || runnerError != "" {
		t.Fatalf("failed block cleanup result = %d/%q", code, runnerError)
	}

	code, runnerError = codeWithCleanupFailure(0, "")
	if code != 3 || runnerError != "cleanup_failed" {
		t.Fatalf("successful block cleanup result = %d/%q", code, runnerError)
	}
}

func TestLocalRunnerFailsClearlyOutsideGit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "```sh setupproof id=run\ntrue\n```\n")

	code, stdout, stderr := runLocal(t, dir, planning.Request{CWD: dir, Positional: []string{"README.md"}}, Options{})
	if code != 2 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "requires a Git working tree") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestSafeLocaleAllowlist(t *testing.T) {
	for _, value := range []string{"C.UTF-8", "C.utf8", "en_US.UTF-8", "POSIX.UTF-8"} {
		if !safeLocale(value) {
			t.Fatalf("safeLocale(%q) = false", value)
		}
	}
	for _, value := range []string{"", "fr_FR.UTF-8", "X; rm -rf $HOME UTF-8"} {
		if safeLocale(value) {
			t.Fatalf("safeLocale(%q) = true", value)
		}
	}
}

func runLocal(t *testing.T, dir string, req planning.Request, opts Options) (int, string, string) {
	t.Helper()
	if req.CWD == "" {
		req.CWD = dir
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(req, opts, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	dir := t.TempDir()
	runGit(t, dir, "init")
	return dir
}

func gitAdd(t *testing.T, dir string, paths ...string) {
	t.Helper()
	args := append([]string{"add", "--"}, paths...)
	runGit(t, dir, args...)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func writeFile(t *testing.T, root string, rel string, contents string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func keptWorkspacePath(t *testing.T, stderr string) string {
	t.Helper()
	re := regexp.MustCompile(`at ([^;\n]+);`)
	matches := re.FindStringSubmatch(stderr)
	if len(matches) != 2 {
		t.Fatalf("kept workspace path missing from stderr: %q", stderr)
	}
	return matches[1]
}

func assertNoWorkspaces(t *testing.T, parent string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(parent, "setupproof-local-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("workspace cleanup mismatch: matches=%#v err=%v", matches, err)
	}
}

func blockLineContains(output string, blockID string, want string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, blockID) && strings.Contains(line, want) {
			return true
		}
	}
	return false
}
