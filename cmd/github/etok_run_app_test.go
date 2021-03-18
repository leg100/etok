package github

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/v31/github"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestEtokRunApp(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		objs  []runtime.Object
		event func(*testutil.T, string, string) interface{}
		// wanted number of etok runs
		wantRuns func(*testutil.T, []*etokRun)
	}{
		{
			name: "checksuite requested event",
			event: func(t *testutil.T, url, sha string) interface{} {
				return &github.CheckSuiteEvent{
					Action: github.String("requested"),
					CheckSuite: &github.CheckSuite{
						HeadBranch: github.String("changes"),
						HeadSHA:    &sha,
					},
					Repo: &github.Repository{
						CloneURL: github.String("file://" + url),
						Name:     github.String("myrepo"),
						Owner: &github.User{
							Login: github.String("bob"),
						},
					},
				}
			},
			objs: []runtime.Object{
				testobj.Workspace("default", "default", testobj.WithRepository("bob/myrepo"), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
			},
			wantRuns: func(t *testutil.T, runs []*etokRun) {
				assert.Equal(t, 1, len(runs))
			},
		},
		{
			name: "checkrun rerequested event",
			event: func(t *testutil.T, url, sha string) interface{} {
				return &github.CheckRunEvent{
					Action: github.String("rerequested"),
					CheckRun: &github.CheckRun{
						CheckSuite: &github.CheckSuite{
							HeadBranch: github.String("changes"),
							HeadSHA:    &sha,
						},
						ExternalID: github.String("default/run-12345"),
					},
					Repo: &github.Repository{
						CloneURL: github.String("file://" + url),
						Name:     github.String("myrepo"),
						Owner: &github.User{
							Login: github.String("bob"),
						},
					},
				}
			},
			objs: []runtime.Object{
				testobj.Run("default", "run-12345", "plan", testobj.WithWorkspace("default")),
				testobj.Workspace("default", "default", testobj.WithRepository("bob/myrepo"), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
			},
			wantRuns: func(t *testutil.T, runs []*etokRun) {
				assert.Equal(t, 1, len(runs))
			},
		},
		{
			name: "checkrun requested_action plan event",
			event: func(t *testutil.T, url, sha string) interface{} {
				return &github.CheckRunEvent{
					Action: github.String("requested_action"),
					CheckRun: &github.CheckRun{
						CheckSuite: &github.CheckSuite{
							HeadBranch: github.String("changes"),
							HeadSHA:    &sha,
						},
						ExternalID: github.String("default/run-12345"),
					},
					Repo: &github.Repository{
						CloneURL: github.String("file://" + url),
						Name:     github.String("myrepo"),
						Owner: &github.User{
							Login: github.String("bob"),
						},
					},
					RequestedAction: &github.RequestedAction{
						Identifier: "plan",
					},
				}
			},
			objs: []runtime.Object{
				testobj.Run("default", "run-12345", "plan", testobj.WithWorkspace("default")),
				testobj.Workspace("default", "default", testobj.WithRepository("bob/myrepo"), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
			},
			wantRuns: func(t *testutil.T, runs []*etokRun) {
				assert.Equal(t, 1, len(runs))
			},
		},
		{
			name: "checkrun requested_action apply event",
			event: func(t *testutil.T, url, sha string) interface{} {
				return &github.CheckRunEvent{
					Action: github.String("requested_action"),
					CheckRun: &github.CheckRun{
						CheckSuite: &github.CheckSuite{
							HeadBranch: github.String("changes"),
							HeadSHA:    &sha,
						},
						ExternalID: github.String("default/run-12345"),
					},
					Repo: &github.Repository{
						CloneURL: github.String("file://" + url),
						Name:     github.String("myrepo"),
						Owner: &github.User{
							Login: github.String("bob"),
						},
					},
					RequestedAction: &github.RequestedAction{
						Identifier: "apply",
					},
				}
			},
			objs: []runtime.Object{
				testobj.Run("default", "run-12345", "plan", testobj.WithWorkspace("default")),
				testobj.Workspace("default", "default", testobj.WithRepository("bob/myrepo"), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
			},
			wantRuns: func(t *testutil.T, runs []*etokRun) {
				assert.Equal(t, 1, len(runs))
				assert.Equal(t, "apply", runs[0].command)
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Create k8s clients
			cc := etokclient.NewFakeClientCreator(tt.objs...)
			client, err := cc.Create("")
			require.NoError(t, err)

			// Create a local mock of the upstream gh repo
			repo, sha := initializeRepo(t, "fixtures/repo")

			// Construct event with mock repo details
			event := tt.event(t, repo, sha)

			app := newEtokRunApp(client, etokAppOptions{
				cloneDir: t.NewTempDir().Root(),
			})

			runs, err := app.createEtokRuns(&fakeTokenRefresher{}, event)
			require.NoError(t, err)

			tt.wantRuns(t, runs)
		})
	}
}

func initializeRepo(t *testutil.T, seed string) (string, string) {
	// Create a temp dir for the repo. Workspaces in the test use the repository
	// identifier "bob/myrepo", so we need to ensure the repo url matches this,
	// i.e. file://tmp/.../bob/myrepo.git
	tmpdir := t.NewTempDir().Mkdir("bob/myrepo.git")
	repo := filepath.Join(tmpdir.Root(), "bob", "myrepo.git")

	seedAbsPath, err := filepath.Abs(seed)
	require.NoError(t, err)

	runCmdInRepo(t, "", "cp", "-a", seedAbsPath+"/.", repo)

	runCmdInRepo(t, repo, "git", "init")
	runCmdInRepo(t, repo, "touch", ".gitkeep")
	runCmdInRepo(t, repo, "git", "add", ".gitkeep")

	runCmdInRepo(t, repo, "git", "config", "--local", "user.email", "etok@etok.dev")
	runCmdInRepo(t, repo, "git", "config", "--local", "user.name", "etok")
	runCmdInRepo(t, repo, "git", "commit", "-m", "initial commit")
	runCmdInRepo(t, repo, "git", "checkout", "-b", "changes")
	runCmdInRepo(t, repo, "git", "add", ".")
	runCmdInRepo(t, repo, "git", "commit", "-am", "changes commit")
	headSHA := runCmdInRepo(t, repo, "git", "rev-parse", "HEAD")
	headSHA = strings.Trim(headSHA, "\n")

	return repo, headSHA
}

func runCmdInRepo(t *testutil.T, dir string, name string, args ...string) string {
	cpCmd := exec.Command(name, args...)
	cpCmd.Dir = dir
	cpOut, err := cpCmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s %s failed: %s", name, args, cpOut)
	}
	return string(cpOut)
}
