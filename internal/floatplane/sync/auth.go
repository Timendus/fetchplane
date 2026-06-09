package sync

import (
	"fmt"
	"log"
	"time"

	"github.com/Timendus/fetchplane/internal/floatplane"
)

func (s *Service) ensureAuthenticated() error {
	if s.state.Auth.ClientID != "" && s.state.Auth.ClientID != s.cfg.OAuthClientID {
		log.Printf("OAuth client ID changed from %s to %s; clearing stored tokens", s.state.Auth.ClientID, s.cfg.OAuthClientID)
		s.state.ClearAuth()
	}

	if accessToken := s.state.AccessToken(); accessToken != "" {
		expiresAt := s.state.TokenExpiresAt()
		if time.Now().Add(60 * time.Second).Before(expiresAt) {
			s.client.SetAccessToken(accessToken)
			return nil
		}
	}

	if refreshToken := s.state.RefreshToken(); refreshToken != "" {
		token, err := s.client.RefreshToken(s.cfg.OAuthClientID, refreshToken)
		if err == nil {
			if err := s.saveToken(token); err != nil {
				return err
			}
			return nil
		}
		log.Printf("refresh failed, starting new device login: %v", err)
		s.state.ClearAuth()
	}

	deviceAuth, err := s.client.BeginDeviceAuthorization(s.cfg.OAuthClientID, s.cfg.OAuthScope)
	if err != nil {
		return err
	}

	log.Printf("authorize FetchPlane by visiting %s", firstNonEmpty(deviceAuth.VerificationURIComplete, deviceAuth.VerificationURI))
	log.Printf("enter code: %s", deviceAuth.UserCode)

	interval := time.Duration(deviceAuth.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(deviceAuth.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		token, oauthErr, err := s.client.PollDeviceAuthorization(s.cfg.OAuthClientID, deviceAuth.DeviceCode)
		if err != nil {
			return err
		}
		if token != nil {
			if err := s.saveToken(token); err != nil {
				return err
			}
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
			return fmt.Errorf("device authorization failed: %s", oauthErr.ErrorDescription)
		}
	}

	return fmt.Errorf("device authorization timed out after %ds", deviceAuth.ExpiresIn)
}

func (s *Service) saveToken(token *floatplane.OAuthToken) error {
	expiresIn := token.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 1800
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	s.state.SetAuth(s.cfg.OAuthClientID, token.TokenType, token.Scope, token.AccessToken, token.RefreshToken, expiresAt)
	s.client.SetAccessToken(token.AccessToken)
	if err := s.state.Save(s.cfg.SessionFile); err != nil {
		return fmt.Errorf("session save error after auth: %w", err)
	}
	return nil
}
