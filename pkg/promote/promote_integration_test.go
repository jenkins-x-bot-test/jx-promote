// +build integration

package promote_test

import (
	"context"
	"strings"
	"testing"

	v1 "github.com/jenkins-x/jx-api/pkg/apis/jenkins.io/v1"
	v1fake "github.com/jenkins-x/jx-api/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-apps/pkg/jxapps"
	"github.com/jenkins-x/jx-promote/pkg/fakes/fakeauth"
	"github.com/jenkins-x/jx-promote/pkg/fakes/fakegit"
	"github.com/jenkins-x/jx-promote/pkg/promote"
	"github.com/jenkins-x/jx-promote/pkg/testhelpers"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
)

// PromoteTestCase a test case of promote
type PromoteTestCase struct {
	name   string
	gitURL string
	gitRef string
	remote bool
}

func TestPromoteIntegrationHelmfile(t *testing.T) {
	AssertPromoteIntegration(t, PromoteTestCase{
		gitURL: "https://github.com/jx3-gitops-repositories/jx3-gke-terraform-vault",
	})
}

func TestPromoteIntegrationJXApps(t *testing.T) {
	// lets deprecate the jx-apps.yml approach
	t.SkipNow()
	AssertPromoteIntegration(t, PromoteTestCase{
		gitURL: "https://github.com/jstrachan/environment-fake-dev",
	})
}

func TestPromoteIntegrationMakefileKpt(t *testing.T) {
	AssertPromoteIntegration(t, PromoteTestCase{
		gitURL: "https://github.com/jstrachan/env-test-promote-makefile",
	})
}

// AssertPromoteIntegration asserts the test cases work
func AssertPromoteIntegration(t *testing.T, testCases ...PromoteTestCase) {
	version := "1.2.3"
	appName := "myapp"
	envName := "staging"
	ns := "jx"

	for _, tc := range testCases {
		_, po := promote.NewCmdPromote()
		name := tc.name
		if name == "" {
			name = tc.gitURL
		}
		po.Application = appName
		po.Version = version
		po.Environment = envName

		po.NoPoll = true
		po.BatchMode = true
		po.AuthConfigService = fakeauth.NewFakeAuthConfigService(t, "jstrachan", "dummytoken", "https://github.com")
		po.GitKind = "fake"
		po.Gitter = fakegit.NewGitFakeClone()
		po.AppGitURL = "https://github.com/myorg/myapp.git"

		targetFullName := "jenkins-x/default-environment-helmfile"

		devEnv, err := testhelpers.CreateTestDevEnvironment(ns)
		require.NoError(t, err, "failed to create dev environment")

		kubeObjects := []runtime.Object{
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
					Labels: map[string]string{
						"tag":  "",
						"team": "jx",
						"env":  "dev",
					},
				},
			},
		}
		jxObjects := []runtime.Object{
			devEnv,
			&v1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envName,
					Namespace: ns,
				},
				Spec: v1.EnvironmentSpec{
					Label:             strings.Title(envName),
					Namespace:         "jx-" + envName,
					PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
					Source: v1.EnvironmentRepository{
						Kind: v1.EnvironmentRepositoryTypeGit,
						URL:  tc.gitURL,
						Ref:  tc.gitRef,
					},
					Order:          0,
					Kind:           "",
					PullRequestURL: "",
					TeamSettings:   v1.TeamSettings{},
					RemoteCluster:  tc.remote,
				},
			},
		}

		po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
		po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
		po.Namespace = ns
		po.DevEnvContext.VersionResolver = testhelpers.CreateTestVersionResolver(t)

		err = po.Run()
		require.NoError(t, err, "failed test %s s", name)

		//testhelpers.AssertTextFilesEqual(t, filepath.Join(expectedDir, "jx-apps.yml"), filepath.Join(resultDir, "jx-apps.yml"), name)

		scmClient := po.ScmClient
		require.NotNil(t, scmClient, "no ScmClient created")
		ctx := context.Background()
		pr, _, err := scmClient.PullRequests.Find(ctx, targetFullName, 1)
		require.NoError(t, err, "failed to find repository %s", targetFullName)
		assert.NotNil(t, pr, "nil pr %s", targetFullName)

		t.Logf("created PullRequest %s", pr.Link)
		t.Logf("PR title: %s", pr.Title)
		t.Logf("PR body: %s", pr.Body)

	}
}

func TestPromoteIntegrationLocalEnvironmentWithNoGitURL(t *testing.T) {
	version := "1.2.3"
	appName := "myapp"
	envName := "staging"
	ns := "jx"

	_, po := promote.NewCmdPromote()
	name := "staging-no-git-url"
	po.Application = appName
	po.Version = version
	po.Environment = envName

	po.NoPoll = true
	po.BatchMode = true
	po.AuthConfigService = fakeauth.NewFakeAuthConfigService(t, "jstrachan", "dummytoken", "https://github.com")
	po.GitKind = "fake"
	po.Gitter = fakegit.NewGitFakeClone()
	po.AppGitURL = "https://github.com/myorg/myapp.git"

	targetFullName := "jenkins-x/default-environment-helmfile"

	devEnv, err := testhelpers.CreateTestDevEnvironment(ns)
	require.NoError(t, err, "failed to create dev environment")
	devEnv.Spec.Source.URL = "https://github.com/jstrachan/environment-fake-dev"

	kubeObjects := []runtime.Object{
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"tag":  "",
					"team": "jx",
					"env":  "dev",
				},
			},
		},
	}
	promoteNamespace := "jx-" + envName
	jxObjects := []runtime.Object{
		devEnv,
		&v1.Environment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      envName,
				Namespace: ns,
			},
			Spec: v1.EnvironmentSpec{
				Label:             strings.Title(envName),
				Namespace:         promoteNamespace,
				PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
				Order:             0,
				Kind:              "",
				PullRequestURL:    "",
				TeamSettings:      v1.TeamSettings{},
				RemoteCluster:     false,
			},
		},
	}

	po.KubeClient = fake.NewSimpleClientset(kubeObjects...)
	po.JXClient = v1fake.NewSimpleClientset(jxObjects...)
	po.Namespace = ns
	po.DevEnvContext.VersionResolver = testhelpers.CreateTestVersionResolver(t)

	err = po.Run()
	require.NoError(t, err, "failed test %s s", name)

	outDir := po.OutDir
	require.DirExists(t, outDir, "failed to create OutDir")
	appsConfig, _, err := jxapps.LoadAppConfig(outDir)
	require.NoError(t, err, "failed to load jx-apps.yml from outDir %s", outDir)
	foundApp := false
	appChart := "jenkins-x/" + appName
	for _, app := range appsConfig.Apps {
		if app.Name == appChart {
			foundApp = true
			assert.Equal(t, promoteNamespace, app.Namespace, "generated jx-apps.yml name %s in dir %s", appChart, outDir)
		}
	}
	assert.True(t, foundApp, "could not find app %s in the generated jx-apps.yml file in outDir %s", appChart, outDir)

	scmClient := po.ScmClient
	require.NotNil(t, scmClient, "no ScmClient created")
	ctx := context.Background()
	pr, _, err := scmClient.PullRequests.Find(ctx, targetFullName, 1)
	require.NoError(t, err, "failed to find repository %s", targetFullName)
	assert.NotNil(t, pr, "nil pr %s", targetFullName)

	t.Logf("created PullRequest %s", pr.Link)
	t.Logf("PR title: %s", pr.Title)
	t.Logf("PR body: %s", pr.Body)

}
