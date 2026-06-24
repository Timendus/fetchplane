package floatplane

import (
	"fmt"
	"log"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane/api"
)

func (s *Service) EnsureAuthenticated(allowLoginStart bool) (bool, error) {
	currentAuth := s.authState()
	if currentAuth.ClientID != "" && currentAuth.ClientID != s.cfg.OAuthClientID {
		log.Printf("OAuth client ID changed from %s to %s; clearing stored tokens", currentAuth.ClientID, s.cfg.OAuthClientID)
		if err := s.clearAuth(); err != nil {
			return false, err
		}
		currentAuth = s.authState()
	}

	if accessToken := currentAuth.AccessToken; accessToken != "" {
		expiresAt := s.tokenExpiresAt()
		if time.Now().Add(60 * time.Second).Before(expiresAt) {
			s.client.SetAccessToken(accessToken)
			s.setLastAuthError("")
			return true, nil
		}
	}

	if refreshToken := currentAuth.RefreshToken; refreshToken != "" {
		token, err := s.client.RefreshToken(s.cfg.OAuthClientID, refreshToken)
		if err == nil {
			if err := s.saveToken(token); err != nil {
				return false, err
			}
			s.setLastAuthError("")
			return true, nil
		}
		log.Printf("refresh failed, starting new device login: %v", err)
		if err := s.clearAuth(); err != nil {
			return false, err
		}
	}

	if pending := s.currentPendingAuth(); pending != nil {
		return false, Skipf("authentication pending")
	}

	if !allowLoginStart {
		s.setLastAuthError("login required")
		return false, Skipf("login required")
	}

	if _, err := s.StartOrReuseLogin("sync"); err != nil {
		s.setLastAuthError(err.Error())
		return false, err
	}

	return false, Skipf("authentication pending")
}

func (s *Service) waitForPendingAuth() error {
	pending := s.currentPendingAuth()
	if pending == nil {
		return Skipf("no active login session")
	}

	interval := pending.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}

	for time.Now().Before(pending.ExpiresAt) {
		time.Sleep(interval)

		pending = s.currentPendingAuth()
		if pending == nil {
			return Skipf("login session cleared")
		}

		token, oauthErr, err := s.client.PollDeviceAuthorization(s.cfg.OAuthClientID, pending.DeviceCode)
		if err != nil {
			log.Printf("device authorization poll error: %v", err)
			continue
		}
		if token != nil {
			if err := s.saveToken(token); err != nil {
				return err
			}

			s.clearPendingAuth("authenticated")
			s.setLastAuthError("")
			s.scheduleNextSync(time.Now())
			return nil
		}
		if oauthErr == nil {
			continue
		}

		switch oauthErr.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		default:
			err := fmt.Errorf("device authorization failed: %s", firstNonEmpty(oauthErr.ErrorDescription, oauthErr.Error))
			s.clearPendingAuth(err.Error())
			s.setLastAuthError(err.Error())
			return err
		}
	}

	err := fmt.Errorf("device authorization timed out")
	s.clearPendingAuth(err.Error())
	s.setLastAuthError(err.Error())
	return Skipf("%s", err.Error())
}

func (s *Service) clearAuth() error {
	s.mu.Lock()
	s.state.ClearAuth()
	err := s.state.Save(s.cfg.SessionFile)
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("session save error after clearing auth: %w", err)
	}
	return nil
}

func (s *Service) saveToken(token *api.OAuthToken) error {
	expiresIn := token.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 1800
	}

	refreshToken := token.RefreshToken
	if refreshToken == "" {
		refreshToken = s.refreshToken()
	}

	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	s.mu.Lock()
	s.state.SetAuth(s.cfg.OAuthClientID, token.TokenType, token.Scope, token.AccessToken, refreshToken, expiresAt)
	s.client.SetAccessToken(token.AccessToken)
	err := s.state.Save(s.cfg.SessionFile)
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("session save error after auth: %w", err)
	}
	return nil
}
