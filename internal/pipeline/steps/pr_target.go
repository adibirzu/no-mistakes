package steps

import (
	"fmt"
	"strings"

	"github.com/kunchenguid/no-mistakes/internal/pipeline"
	"github.com/kunchenguid/no-mistakes/internal/scm"
	"github.com/kunchenguid/no-mistakes/internal/scm/github"
)

// ValidateRunPRTargetURL validates the explicit GitHub-only target accepted
// by AXI. Other SCMs fail closed in resolveExplicitPRTarget rather than being
// silently downgraded to branch discovery.
func ValidateRunPRTargetURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	return github.CanonicalPRURL(trimmed)
}

// explicitPRTargetExpectation is the immutable identity a caller must prove
// before a target-bound run can write to an already-open pull request.
type explicitPRTargetExpectation struct {
	RequestedURL  string
	RepositoryURL string
	PushURL       string
	BaseBranch    string
	HeadBranch    string
}

// verifyExplicitPRTarget keeps target identity checks independent from the
// provider transport. A matching PR number is deliberately insufficient: an
// operator-selected PR must still be open, belong to this repository, source
// this run's branch from the configured push remote, and target this run's
// effective base branch.
func verifyExplicitPRTarget(expected explicitPRTargetExpectation, target *scm.PRTarget) (*scm.PR, error) {
	if target == nil {
		return nil, fmt.Errorf("explicit pull request target %q was not found", expected.RequestedURL)
	}
	requested := strings.TrimRight(strings.TrimSpace(expected.RequestedURL), "/")
	actual := strings.TrimRight(strings.TrimSpace(target.URL), "/")
	if requested == "" || actual == "" || !strings.EqualFold(requested, actual) {
		return nil, fmt.Errorf("explicit pull request target is ambiguous: requested %q, provider returned %q", expected.RequestedURL, target.URL)
	}
	if target.State != scm.PRStateOpen {
		return nil, fmt.Errorf("explicit pull request target %s is not open (state %s)", requested, strings.ToLower(string(target.State)))
	}
	if !sameRepositoryURL(target.BaseRepositoryURL, expected.RepositoryURL) {
		return nil, fmt.Errorf("explicit pull request target %s repository %q does not match configured repository %q", requested, target.BaseRepositoryURL, expected.RepositoryURL)
	}
	if strings.TrimSpace(target.BaseBranch) != strings.TrimSpace(expected.BaseBranch) {
		return nil, fmt.Errorf("explicit pull request target %s base branch %q does not match %q", requested, target.BaseBranch, expected.BaseBranch)
	}
	if strings.TrimSpace(target.HeadBranch) != strings.TrimSpace(expected.HeadBranch) {
		return nil, fmt.Errorf("explicit pull request target %s head branch %q does not match %q", requested, target.HeadBranch, expected.HeadBranch)
	}
	if !sameRepositoryURL(target.HeadRepositoryURL, expected.PushURL) {
		return nil, fmt.Errorf("explicit pull request target %s head repository %q does not match configured push remote %q", requested, target.HeadRepositoryURL, expected.PushURL)
	}
	return &target.PR, nil
}

func sameRepositoryURL(left, right string) bool {
	leftHost, rightHost := scm.ExtractHost(left), scm.ExtractHost(right)
	leftPath, rightPath := scm.RepoPath(left), scm.RepoPath(right)
	return leftHost != "" && rightHost != "" && leftPath != "" && rightPath != "" &&
		strings.EqualFold(leftHost, rightHost) && strings.EqualFold(leftPath, rightPath)
}

// resolveExplicitPRTarget is intentionally opt-in. An empty run target keeps
// the legacy discovery/create behavior; a non-empty target either verifies the
// named object or returns an error before a caller can write or publish.
func resolveExplicitPRTarget(sctx *pipeline.StepContext, host scm.Host) (*scm.PR, bool, error) {
	targetURL := runPRTargetURL(sctx)
	if targetURL == "" {
		return nil, false, nil
	}
	reader, ok := host.(scm.PRTargetReader)
	if !ok {
		return nil, true, fmt.Errorf("explicit pull request target %q is not supported by %s", targetURL, host.Provider())
	}
	target, err := reader.GetPRTarget(sctx.Ctx, targetURL)
	if err != nil {
		return nil, true, fmt.Errorf("read explicit pull request target %q: %w", targetURL, err)
	}
	branch := strings.TrimPrefix(strings.TrimSpace(sctx.Run.Branch), "refs/heads/")
	pr, err := verifyExplicitPRTarget(explicitPRTargetExpectation{
		RequestedURL:  targetURL,
		RepositoryURL: sctx.Repo.UpstreamURL,
		PushURL:       resolvePushURL(sctx),
		BaseBranch:    effectivePRBaseBranch(sctx),
		HeadBranch:    branch,
	}, target)
	if err != nil {
		return nil, true, err
	}
	return pr, true, nil
}

func runPRTargetURL(sctx *pipeline.StepContext) string {
	if sctx == nil || sctx.Run == nil || sctx.Run.PRTargetURL == nil {
		return ""
	}
	return strings.TrimSpace(*sctx.Run.PRTargetURL)
}
