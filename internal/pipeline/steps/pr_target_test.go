package steps

import (
	"strings"
	"testing"

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

func clonePRTarget(source *scm.PRTarget, mutate func(*scm.PRTarget)) *scm.PRTarget {
	clone := *source
	mutate(&clone)
	return &clone
}
