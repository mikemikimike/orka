/*
Copyright (c) 2026.

MIT License - see LICENSE file for details.
*/

package api

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/orka-agents/orka/internal/ahphost"
)

func TestAHPWorkspaceIdle(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(-5 * time.Hour))}}
	require.True(t, ahpWorkspaceIdle(now, time.Time{}, pod, 4*time.Hour))
	require.False(t, ahpWorkspaceIdle(now, now.Add(-time.Hour), pod, 4*time.Hour), "this process saw a recent use")
	pod.Annotations = map[string]string{ahpWorkspaceActiveAnnotation: now.Add(-30 * time.Minute).Format(time.RFC3339)}
	require.False(t, ahpWorkspaceIdle(now, time.Time{}, pod, 4*time.Hour), "the annotation survives restarts")
	pod.Annotations = nil
	pod.CreationTimestamp = metav1.NewTime(now.Add(-time.Hour))
	require.False(t, ahpWorkspaceIdle(now, time.Time{}, pod, 4*time.Hour), "a new workspace is not idle")
}

func TestAHPWorkspaceCollectorKeepsWorkspacesInUse(t *testing.T) {
	env := newAHPTestEnv(t)
	backend := env.backend
	backend.cfg.WorkspaceIdleTimeout = time.Hour
	old := time.Now().Add(-48 * time.Hour)
	backend.now = time.Now
	w := &ahpWorkspaces{b: backend, clientset: kubefake.NewClientset(), rest: &rest.Config{}, namespace: "orka-ahp-workspaces", image: "example.com/git@sha256:" + ahpHash("x")}
	backend.workspaces = w
	alice := ahphost.Principal{UserKey: ahphost.UserKey("https://issuer.example", "alice")}
	bob := ahphost.Principal{UserKey: ahphost.UserKey("https://issuer.example", "bob")}
	for _, p := range []ahphost.Principal{alice, bob} {
		pod := w.podSpec(p)
		pod.CreationTimestamp = metav1.NewTime(old)
		require.NoError(t, env.kube.Create(context.Background(), pod))
	}
	// Bob has a shell open; Alice has nothing running.
	backend.mu.Lock()
	backend.shells[ahpTerminalKey(bob.UserKey, "ahp-terminal:/b")] = &ahpTerminal{}
	backend.mu.Unlock()

	w.collectOnce(context.Background())
	err := env.kube.Get(context.Background(), types.NamespacedName{Namespace: w.namespace, Name: ahpWorkspacePodName(alice.UserKey)}, &corev1.Pod{})
	require.True(t, apierrors.IsNotFound(err), "an idle workspace with nothing running is removed")
	require.NoError(t, env.kube.Get(context.Background(), types.NamespacedName{Namespace: w.namespace, Name: ahpWorkspacePodName(bob.UserKey)}, &corev1.Pod{}),
		"a workspace with a shell stays")
}

// A branch's changes are found from its base, or without one, as for a
// repository off github.com, from the default branch the clone found.
func TestAHPBranchForkScript(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@example.com", "-c", "user.name=t", "-c", "init.defaultBranch=main"}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	remote := filepath.Join(t.TempDir(), "remote.git")
	seed, checkout := t.TempDir(), filepath.Join(t.TempDir(), "checkout")
	require.NoError(t, os.MkdirAll(remote, 0o755))
	git(remote, "init", "--bare")
	git(seed, "init")
	require.NoError(t, os.WriteFile(filepath.Join(seed, "a"), []byte("a\n"), 0o600))
	git(seed, "add", "a")
	git(seed, "commit", "-m", "a")
	git(seed, "push", remote, "main")
	git(filepath.Dir(checkout), "clone", "--quiet", remote, checkout)
	fork := git(checkout, "rev-parse", "HEAD")
	git(checkout, "checkout", "--quiet", "-b", "session")
	require.NoError(t, os.WriteFile(filepath.Join(checkout, "b"), []byte("b\n"), 0o600))
	git(checkout, "add", "b")
	git(checkout, "commit", "-m", "b")
	for _, base := range []string{"main", ""} {
		out, err := exec.Command("/bin/sh", "-c", ahpBranchForkScript, "sh", checkout, base).Output()
		require.NoError(t, err, "base %q", base)
		require.Equal(t, fork+"\x00A\x00b\x00", string(out), "base %q", base)
	}
}

// The scripts run in the workspace; run them here against real repositories.
// A checkout already at a project's path is reused only for its repository,
// and a session's branch a disposed session left unpublished is reused.
func TestAHPCloneAndWorktreeScripts(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@example.com", "-c", "user.name=t", "-c", "init.defaultBranch=main"}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	script := func(body string, args ...string) (string, error) {
		out, err := exec.Command("/bin/sh", append([]string{"-c", body, "sh"}, args...)...).CombinedOutput()
		return string(out), err
	}
	root := t.TempDir()
	seed := t.TempDir()
	git(seed, "init")
	require.NoError(t, os.WriteFile(filepath.Join(seed, "a"), []byte("a\n"), 0o600))
	git(seed, "add", "a")
	git(seed, "commit", "-m", "a")
	remotes := map[string]string{}
	for _, name := range []string{"one", "other"} {
		remotes[name] = filepath.Join(root, name+".git")
		require.NoError(t, os.MkdirAll(remotes[name], 0o755))
		git(remotes[name], "init", "--bare")
		git(seed, "push", "--quiet", remotes[name], "main")
	}
	checkout, clones := filepath.Join(root, "src", "octo", "widgets"), filepath.Join(root, ".clones")

	_, err := script(ahpCloneScript, checkout, remotes["one"], clones)
	require.NoError(t, err)
	_, err = script(ahpCloneScript, checkout, remotes["one"], clones)
	require.NoError(t, err, "the project's own checkout is reused")
	// A URL rewrite the person set up does not hide the checkout's origin.
	git(checkout, "config", "url."+remotes["other"]+".insteadOf", remotes["one"])
	_, err = script(ahpCloneScript, checkout, remotes["one"], clones)
	require.NoError(t, err, "a rewritten origin is still the project's")
	git(checkout, "config", "--unset", "url."+remotes["other"]+".insteadOf")
	out, err := script(ahpCloneScript, checkout, remotes["other"], clones)
	require.Error(t, err, "another repository's checkout is not reused")
	require.Contains(t, out, "holds a checkout of another repository")

	worktree := filepath.Join(root, "worktrees", "octo", "widgets", "feature")
	_, err = script(ahpWorktreeScript, checkout, worktree, "feature", "main")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "b"), []byte("b\n"), 0o600))
	git(worktree, "add", "b")
	git(worktree, "commit", "-m", "b")
	kept := git(worktree, "rev-parse", "HEAD")
	// Disposing of the session removes its worktree; its branch stays.
	git(checkout, "worktree", "remove", "--force", worktree)
	out, err = script(ahpWorktreeScript, checkout, worktree, "feature", "main")
	require.NoError(t, err, out)
	require.Equal(t, kept, git(worktree, "rev-parse", "HEAD"), "the unpublished branch is reused as it was")
}

// The script runs in the workspace; run it here against real repositories.
func TestAHPHoldsWorkScript(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	// Git reports real paths; the temporary directory may be a symlink.
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@example.com", "-c", "user.name=t", "-c", "init.defaultBranch=main"}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	holds := func() string {
		out, err := exec.Command("/bin/sh", "-c", ahpHoldsWorkScript, "sh", root).Output()
		require.NoError(t, err)
		return strings.TrimSpace(string(out))
	}
	remote := filepath.Join(t.TempDir(), "remote.git")
	require.NoError(t, os.MkdirAll(remote, 0o755))
	git(remote, "init", "--bare")
	project := filepath.Join(root, "src", "o", "r")
	require.NoError(t, os.MkdirAll(project, 0o755))
	git(project, "init")
	require.NoError(t, os.WriteFile(filepath.Join(project, "a"), []byte("a\n"), 0o600))
	git(project, "add", "a")
	git(project, "commit", "-m", "a")
	git(project, "remote", "add", "origin", remote)
	git(project, "push", "-u", "origin", "main")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "worktrees"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".home"), 0o755))
	require.Equal(t, "done", holds(), "everything is pushed")

	// A local branch with no upstream holds a commit, even when it is not
	// checked out.
	git(project, "switch", "--no-track", "-c", "scratch")
	require.NoError(t, os.WriteFile(filepath.Join(project, "b"), []byte("b\n"), 0o600))
	git(project, "add", "b")
	git(project, "commit", "-m", "b")
	git(project, "switch", "main")
	require.Contains(t, holds(), "unpushed "+project)
	git(project, "push", "origin", "scratch")
	require.Equal(t, "done", holds())

	// An uncommitted file in a worktree, and a file outside the checkouts.
	worktree := filepath.Join(root, "worktrees", "o", "r", "s")
	git(project, "worktree", "add", "-b", "s", worktree, "main")
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "c"), []byte("c\n"), 0o600))
	require.Contains(t, holds(), "changed "+worktree)
	require.NoError(t, os.Remove(filepath.Join(worktree, "c")))
	// Untracked files show even when the repository hides them.
	git(project, "config", "status.showUntrackedFiles", "no")
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "d"), []byte("d\n"), 0o600))
	require.Contains(t, holds(), "changed "+worktree)
	require.NoError(t, os.Remove(filepath.Join(worktree, "d")))
	require.Equal(t, "done", holds())

	// A commit on a detached HEAD in a worktree.
	detached := filepath.Join(root, "worktrees", "o", "r", "detached")
	git(project, "worktree", "add", "--detach", detached, "main")
	require.NoError(t, os.WriteFile(filepath.Join(detached, "e"), []byte("e\n"), 0o600))
	git(detached, "add", "e")
	git(detached, "commit", "-m", "e")
	require.Contains(t, holds(), "unpushed "+project, "every worktree's HEAD counts")
	git(project, "worktree", "remove", "--force", detached)

	require.NoError(t, os.WriteFile(filepath.Join(root, ".home", ".ash_history"), []byte("ls\n"), 0o600))
	require.Equal(t, "done", holds(), "shell history is not kept")
	require.NoError(t, os.WriteFile(filepath.Join(root, ".home", "notes.txt"), []byte("n\n"), 0o600))
	require.Contains(t, holds(), "other "+filepath.Join(root, ".home", "notes.txt"), "other files in HOME are kept")
	require.NoError(t, os.Remove(filepath.Join(root, ".home", "notes.txt")))

	// A directory the scan cannot read keeps the workspace.
	locked := filepath.Join(root, "locked")
	require.NoError(t, os.MkdirAll(locked, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(locked, "f"), []byte("f\n"), 0o600))
	require.NoError(t, os.Chmod(locked, 0))
	require.Contains(t, holds(), "error find")
	require.NoError(t, os.Chmod(locked, 0o755))
	require.NoError(t, os.RemoveAll(locked))

	// A tag keeps an unpublished commit that no branch has.
	git(project, "switch", "--detach", "main")
	require.NoError(t, os.WriteFile(filepath.Join(project, "g"), []byte("g\n"), 0o600))
	git(project, "add", "g")
	git(project, "commit", "-m", "g")
	git(project, "tag", "kept")
	git(project, "switch", "main")
	require.Contains(t, holds(), "unpushed "+project)
	git(project, "tag", "-d", "kept")
	require.Equal(t, "done", holds(), "commits only the reflog keeps are not kept")

	// A file git was told not to check.
	git(project, "update-index", "--assume-unchanged", "a")
	require.Contains(t, holds(), "flagged "+project)
	git(project, "update-index", "--no-assume-unchanged", "a")

	// An unreadable directory inside a checkout.
	hidden := filepath.Join(project, "hidden")
	require.NoError(t, os.MkdirAll(hidden, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hidden, "h"), []byte("h\n"), 0o600))
	require.NoError(t, os.Chmod(hidden, 0))
	require.Contains(t, holds(), "error "+project)
	require.NoError(t, os.Chmod(hidden, 0o755))
	require.NoError(t, os.RemoveAll(hidden))
	require.Equal(t, "done", holds())

	// Any submodule keeps the workspace.
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".git", "modules"), 0o755))
	require.Contains(t, holds(), "submodules "+project)
	require.NoError(t, os.RemoveAll(filepath.Join(project, ".git", "modules")))
	for _, stray := range []string{filepath.Join(root, "notes.txt"), filepath.Join(root, "src", "notes.txt"), filepath.Join(root, "worktrees", "o", "x.txt")} {
		require.NoError(t, os.WriteFile(stray, []byte("n\n"), 0o600))
		require.Contains(t, holds(), "other "+stray)
		require.NoError(t, os.Remove(stray))
	}
	require.Equal(t, "done", holds())
}

// A restore that could not clone every project leaves the Pod unmarked, so a
// later use restores it again; one that finished marks it.
func TestAHPWorkspaceRestoreIsMarkedOnlyWhenComplete(t *testing.T) {
	env := newAHPTestEnv(t)
	ctx := ahpCtx(t)
	alice := ahphost.Principal{Issuer: "https://issuer.example", Subject: "alice", Namespace: ahpTestNamespace,
		UserKey: ahphost.UserKey("https://issuer.example", "alice")}
	// Exec reaches no API server here, so every clone fails.
	unreachable := &rest.Config{Host: "http://127.0.0.1:1"}
	clientset, err := kubernetes.NewForConfig(unreachable)
	require.NoError(t, err)
	w := &ahpWorkspaces{b: env.backend, clientset: clientset, rest: unreachable, namespace: "orka-ahp-workspaces",
		image: "example.com/git@sha256:" + ahpHash("x")}
	restored := func(name string) bool {
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: w.namespace}}
		require.NoError(t, env.kube.Create(ctx, pod))
		w.restore(ctx, alice, pod)
		require.NoError(t, env.kube.Get(ctx, types.NamespacedName{Namespace: w.namespace, Name: name}, pod))
		return pod.Annotations[ahpWorkspaceRestoredAnnotation] != ""
	}
	require.True(t, restored("nothing-to-restore"), "a restore with nothing to do is complete")
	_, err = env.backend.addProject(ctx, alice, ahpTestRepo)
	require.NoError(t, err)
	require.False(t, restored("clone-fails"), "a restore whose clone failed is tried again")
}

// A terminal's directory that could not be set up is reported, so the shell
// never opens somewhere else while the client believes it is in the project.
func TestAHPWorkspacePrepareReportsAFailedSetup(t *testing.T) {
	env := newAHPTestEnv(t)
	ctx := ahpCtx(t)
	alice := ahphost.Principal{Issuer: "https://issuer.example", Subject: "alice", Namespace: ahpTestNamespace,
		UserKey: ahphost.UserKey("https://issuer.example", "alice")}
	// Exec reaches no API server here, so the clone fails.
	unreachable := &rest.Config{Host: "http://127.0.0.1:1"}
	clientset, err := kubernetes.NewForConfig(unreachable)
	require.NoError(t, err)
	w := &ahpWorkspaces{b: env.backend, clientset: clientset, rest: unreachable, namespace: "orka-ahp-workspaces",
		image: "example.com/git@sha256:" + ahpHash("x")}
	project, err := env.backend.addProject(ctx, alice, ahpTestRepo)
	require.NoError(t, err)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "workspace", Namespace: w.namespace}}
	require.Error(t, w.prepare(ctx, alice, pod, project.Path), "a project that could not be cloned")
	require.NoError(t, w.prepare(ctx, alice, pod, "/workspace"), "a directory that needs no setup")
}

func TestAHPWorkspaceUseWaitsOutACollection(t *testing.T) {
	env := newAHPTestEnv(t)
	w := &ahpWorkspaces{b: env.backend}
	alice := ahphost.Principal{UserKey: ahphost.UserKey("https://issuer.example", "alice")}
	done, ok := w.beginCollect(alice.UserKey[:40])
	require.True(t, ok)
	_, again := w.beginCollect(alice.UserKey[:40])
	require.False(t, again, "one collection at a time")
	claimed := make(chan error, 1)
	go func() { claimed <- w.claimForUse(context.Background(), alice) }()
	select {
	case err := <-claimed:
		t.Fatalf("a use went ahead while the workspace was being collected: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	done()
	require.NoError(t, <-claimed)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(time.Now().Add(-48 * time.Hour))}}
	env.backend.cfg.WorkspaceIdleTimeout = time.Hour
	require.False(t, w.idle(alice.UserKey[:40], pod), "the use counts as recent")
}
