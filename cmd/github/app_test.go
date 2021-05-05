package github

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestHandleEvent(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		objs           []runtime.Object
		event          func(*testutil.T, string, string) interface{}
		wantCheckRuns  func(*testutil.T, []*check)
		wantRunArchive func(*testutil.T, *v1alpha1.Run, *corev1.ConfigMap)
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
				testobj.Workspace("default", "foo", testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
				testobj.Workspace("default", "bar", testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir2")),
			},
			wantCheckRuns: func(t *testutil.T, checks []*check) {
				assert.Equal(t, 2, len(checks))
			},
		},
		{
			name: "checkrun rerequested plan",
			event: func(t *testutil.T, url, sha string) interface{} {
				return &github.CheckRunEvent{
					Action: github.String("rerequested"),
					CheckRun: &github.CheckRun{
						CheckSuite: &github.CheckSuite{
							ID:         github.Int64(123456),
							HeadBranch: github.String("changes"),
							HeadSHA:    &sha,
						},
						ExternalID: (&CheckMetadata{
							Current:   "run-12345",
							Namespace: "default",
							Command:   "plan",
							Workspace: "default",
						}).ToStringPtr(),
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
				testobj.Workspace("default", "default", testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
				testobj.Run("default", "run-12345", "plan", testobj.WithWorkspace("default"), testobj.WithLabels(
					checkSuiteIDLabelName, "123456",
				)),
			},
			wantRunArchive: func(t *testutil.T, run *v1alpha1.Run, archive *corev1.ConfigMap) {
				assert.NotNil(t, run)
				assert.NotNil(t, archive)
			},
		},
		{
			name: "checkrun requested_action plan event",
			event: func(t *testutil.T, url, sha string) interface{} {
				return &github.CheckRunEvent{
					Action: github.String("requested_action"),
					CheckRun: &github.CheckRun{
						CheckSuite: &github.CheckSuite{
							ID:         github.Int64(123456),
							HeadBranch: github.String("changes"),
							HeadSHA:    &sha,
						},
						ExternalID: (&CheckMetadata{
							Current:   "run-12345",
							Namespace: "default",
							Command:   "plan",
							Workspace: "default",
						}).ToStringPtr(),
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
				testobj.Workspace("default", "default", testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
				testobj.Run("default", "run-12345", "plan", testobj.WithWorkspace("default"), testobj.WithLabels(
					checkSuiteIDLabelName, "123456",
				)),
			},
			wantRunArchive: func(t *testutil.T, run *v1alpha1.Run, archive *corev1.ConfigMap) {
				assert.NotNil(t, run)
				assert.NotNil(t, archive)
			},
		},
		{
			name: "checkrun requested_action apply event",
			event: func(t *testutil.T, url, sha string) interface{} {
				return &github.CheckRunEvent{
					Action: github.String("requested_action"),
					CheckRun: &github.CheckRun{
						CheckSuite: &github.CheckSuite{
							ID:         github.Int64(123456),
							HeadBranch: github.String("changes"),
							HeadSHA:    &sha,
						},
						ExternalID: (&CheckMetadata{
							Current:   "run-12345",
							Namespace: "default",
							Command:   "plan",
							Workspace: "default",
						}).ToStringPtr(),
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
				testobj.Workspace("default", "default", testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
			},
			wantRunArchive: func(t *testutil.T, run *v1alpha1.Run, archive *corev1.ConfigMap) {
				assert.NotNil(t, run)
				assert.NotNil(t, archive)
			},
		},
		{
			name: "checkrun created event",
			event: func(t *testutil.T, url, sha string) interface{} {
				var checkRunId int64 = 123456

				return &github.CheckRunEvent{
					Action: github.String("created"),
					CheckRun: &github.CheckRun{
						ID: &checkRunId,
						CheckSuite: &github.CheckSuite{
							ID:         github.Int64(123456),
							HeadBranch: github.String("changes"),
							HeadSHA:    &sha,
						},
						ExternalID: (&CheckMetadata{
							Current:   "run-12345",
							Namespace: "default",
							Command:   "plan",
							Workspace: "default",
						}).ToStringPtr(),
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
				testobj.Workspace("default", "default", testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Create a local mock of the upstream gh repo
			repo, sha := initializeRepo(t, "fixtures/repo")

			// Connect up workspaces to mock repo
			for _, obj := range tt.objs {
				ws, ok := obj.(*v1alpha1.Workspace)
				if ok {
					ws.Spec.VCS.Repository = "file://" + repo
				}
			}

			// Create k8s clients and populate with mock objs
			cc := etokclient.NewFakeClientCreator(tt.objs...)
			client, err := cc.Create("")
			require.NoError(t, err)

			// Construct event with mock repo details
			event := tt.event(t, repo, sha)

			app := newApp(client, appOptions{
				cloneDir: t.NewTempDir().Root(),
			})

			mgr := &fakeInstallsMgr{}
			require.NoError(t, app.handleEvent(event, mgr))

			if tt.wantCheckRuns != nil {
				tt.wantCheckRuns(t, mgr.checks)
			}

			if tt.wantRunArchive != nil {
				selector := fmt.Sprintf("%s=true", githubTriggeredLabelName)
				runs, err := client.RunsClient("default").List(context.Background(), metav1.ListOptions{LabelSelector: selector})
				require.NoError(t, err)
				require.Equal(t, 1, len(runs.Items))

				archives, err := client.ConfigMapsClient("default").List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				require.Equal(t, 1, len(archives.Items))

				tt.wantRunArchive(t, &runs.Items[0], &archives.Items[0])
			}
		})
	}
}

func TestRunScript(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		command  string
		previous string
		want     string
	}{
		{
			name:     "default",
			id:       "run-12345",
			command:  "plan",
			previous: "",
			want:     "set -e\n\nterraform init -no-color -input=false\n\n\nterraform plan -no-color -input=false -out=/plans/run-12345\n\n",
		},
		{
			name:     "default",
			id:       "run-12345",
			command:  "apply",
			previous: "run-12345",
			want:     "set -e\n\nterraform init -no-color -input=false\n\n\n\nterraform apply -no-color -input=false /plans/run-12345\n",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			assert.Equal(t, tt.want, runScript(tt.id, tt.command, tt.previous))
		})
	}
}

type fakeInstallsMgr struct {
	checks []*check
}

func (m *fakeInstallsMgr) getTokenRefresher(installID int64) (tokenRefresher, error) {
	return &fakeTokenRefresher{}, nil
}

func (m *fakeInstallsMgr) send(_ int64, inv invoker) error {
	m.checks = append(m.checks, inv.(*check))

	return nil
}

type fakeTokenRefresher struct{}

func (tr *fakeTokenRefresher) refreshToken() (string, error) {
	return "token123", nil
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
