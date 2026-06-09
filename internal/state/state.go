package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	Auth AuthState `json:"auth"`
}

type AuthState struct {
	ClientID     string `json:"clientId,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	TokenType    string `json:"tokenType,omitempty"`
	Scope        string `json:"scope,omitempty"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
}

func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return New(), nil
		}
		return nil, err
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func New() *State {
	return &State{}
}

func (s *State) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *State) AccessToken() string {
	return s.Auth.AccessToken
}

func (s *State) RefreshToken() string {
	return s.Auth.RefreshToken
}

func (s *State) TokenExpiresAt() time.Time {
	if s.Auth.ExpiresAt == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s.Auth.ExpiresAt)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (s *State) SetAuth(clientID string, tokenType string, scope string, accessToken string, refreshToken string, expiresAt time.Time) {
	s.Auth = AuthState{
		ClientID:     clientID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    tokenType,
		Scope:        scope,
		ExpiresAt:    expiresAt.UTC().Format(time.RFC3339Nano),
	}
}

func (s *State) ClearAuth() {
	s.Auth = AuthState{}
}
