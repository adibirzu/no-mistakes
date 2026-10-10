package steps

import (
	"os"
	"strings"
	"testing"

	"github.com/kunchenguid/no-mistakes/internal/config"
	"github.com/kunchenguid/no-mistakes/internal/scm"
)

func TestVerifyExplicitPRTarget(t *testing.T) {
	t.Parallel()

	expected := explicitPRTargetExpectation{
		RequestedURL:  "https://github.com/kunchenguid/no-mistakes/pull/37",
		RepositoryURL: "https://github.com/kunchenguid/no-mistakes.git",
		PushURL:       "git@github.com:adibirzu/no-mistakes.git",
		BaseBranch:    "main",
		HeadBranch:    "feature/explicit-pr",
	}
	valid := &scm.PRTarget{
		PR: scm.PR{
			Number:     "37",
			URL:        expected.RequestedURL,
			BaseBranch: expected.BaseBranch,
		},
		State:             scm.PRStateOpen,
		HeadBranch:        expected.HeadBranch,
		BaseRepositoryURL: "https://github.com/kunchenguid/no-mistakes",
		HeadRepositoryURL: "https://github.com/adibirzu/no-mistakes",
	}

	tests := []struct {
		name    string
		target  *scm.PRTarget
		wantErr string
	}{
		{name: "valid binding", target: valid},
		{name: "missing target", wantErr: "not found"},
		{
			name: "ambiguous target identity",
			target: clonePRTarget(valid, func(target *scm.PRTarget) {
				target.URL = "https://github.com/kunchenguid/no-mistakes/pull/38"
			}),
			wantErr: "ambiguous",
		},
		{
			name: "closed target",
			target: clonePRTarget(valid, func(target *scm.PRTarget) {
				target.State = scm.PRStateClosed
			}),
			wantErr: "not open",
		},
		{
			name: "foreign repository",
			target: clonePRTarget(valid, func(target *scm.PRTarget) {
				target.BaseRepositoryURL = "https://github.com/foreign/no-mistakes"
			}),
			wantErr: "repository",
		},
		{
			name: "mismatched base",
			target: clonePRTarget(valid, func(target *scm.PRTarget) {
				target.BaseBranch = "develop"
			}),
			wantErr: "base branch",
		},
		{
			name: "mismatched head",
			target: clonePRTarget(valid, func(target *scm.PRTarget) {
				target.HeadBranch = "other-branch"
			}),
			wantErr: "head branch",
		},
		{
			name: "remote divergence",
			target: clonePRTarget(valid, func(target *scm.PRTarget) {
				target.HeadRepositoryURL = "https://github.com/other/no-mistakes"
			}),
			wantErr: "push remote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := verifyExplicitPRTarget(expected, tt.target)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("verifyExplicitPRTarget() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || got.URL != expected.RequestedURL || got.Number != "37" {
				t.Fatalf("verified PR = %#v, want target %q (#37)", got, expected.RequestedURL)
			}
		})
	}
}

func TestPRStep_ExplicitTargetNeverDiscoversOrCreatesReplacement(t *testing.T) {
	dir, baseSHA, headSHA := setupGitRepo(t)
	const targetURL = "https://github.com/test/repo/pull/42"

	sctx := newTestContextWithDBRecords(t, &mockAgent{name: "test"}, dir, baseSHA, headSHA, config.Commands{})
	sctx.Repo.UpstreamURL = "https://github.com/test/repo"
	sctx.Repo.ForkURL = "https://github.com/test/repo"
	sctx.Run.Branch = "refs/heads/feature"
	sctx.Run.PRTargetURL = strptr(targetURL)
	logFile := t.TempDir() + "/gh.log"
	targetJSON := `{"number":42,"url":"https://github.com/test/repo/pull/42","state":"OPEN","baseRefName":"main","headRefName":"feature","baseRepository":{"url":"https://github.com/test/repo"},"headRepository":{"url":"https://github.com/test/repo"}}`
	sctx.Env = append(fakeCIGH(t, "OPEN", `[]`),
		"FAKE_CLI_LOG="+logFile,
		"FAKE_CLI_PR_TARGET_JSON="+targetJSON,
	)

	if _, err := (&PRStep{}).Execute(sctx); err != nil {
		t.Fatal(err)
	}
	logData, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	transcript := string(logData)
	if !strings.Contains(transcript, "pr view "+targetURL+" --json number,url,state,baseRefName,headRefName,baseRepository,headRepository") {
		t.Fatalf("expected live lookup of selected pull request, got:\n%s", transcript)
	}
	if !strings.Contains(transcript, "pr edit 42") {
		t.Fatalf("expected only the selected pull request to be updated, got:\n%s", transcript)
	}
	if strings.Contains(transcript, "pr list") || strings.Contains(transcript, "pr create") {
		t.Fatalf("explicit target must not use discovery or create a replacement PR, got:\n%s", transcript)
	}
	t.Logf("explicit PR transcript:\n%s", transcript)
}

func clonePRTarget(source *scm.PRTarget, mutate func(*scm.PRTarget)) *scm.PRTarget {
	clone := *source
	mutate(&clone)
	return &clone
}
