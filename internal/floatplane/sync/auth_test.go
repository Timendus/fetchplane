package sync

import (
	"testing"
	"time"

	"github.com/Timendus/fetchplane/internal/config"
	"github.com/Timendus/fetchplane/internal/floatplane"
	"github.com/Timendus/fetchplane/internal/state"
)

func TestSaveTokenPreservesExistingRefreshTokenWhenRefreshResponseOmitsIt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	st := state.New()
	st.SetAuth("client-id", "Bearer", "scope", "old-access", "old-refresh", mustTime(t, "2026-01-01T00:00:00Z"))

	service := New(&config.Config{
		OAuthClientID: "client-id",
		SessionFile:   dir + "/session.json",
	}, floatplane.NewClient(), st)

	err := service.saveToken(&floatplane.OAuthToken{
		AccessToken: "new-access",
		TokenType:   "Bearer",
		Scope:       "scope",
		ExpiresIn:   3600,
	})
	if err != nil {
		t.Fatalf("saveToken returned error: %v", err)
	}

	if got := st.RefreshToken(); got != "old-refresh" {
		t.Fatalf("expected refresh token to be preserved, got %q", got)
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}
