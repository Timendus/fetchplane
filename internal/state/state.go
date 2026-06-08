package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Timendus/fetchplane/internal/api"
)

type State struct {
	Creators    map[string]CreatorState    `json:"creators"`
	Attachments map[string]AttachmentState `json:"attachments"`
	Auth        AuthState                  `json:"auth"`
}

type CreatorState struct {
	LatestReleaseDate string `json:"latestReleaseDate,omitempty"`
}

type AttachmentState struct {
	CreatorID   string `json:"creatorId,omitempty"`
	PostID      string `json:"postId,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ReleaseDate string `json:"releaseDate,omitempty"`
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
	st.ensureMaps()
	return &st, nil
}

func New() *State {
	st := &State{}
	st.ensureMaps()
	return st
}

func (s *State) Save(path string) error {
	s.ensureMaps()

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

func (s *State) HasAttachment(attachmentID string) bool {
	s.ensureMaps()
	_, ok := s.Attachments[attachmentID]
	return ok
}

func (s *State) HasAllAttachments(attachments []api.VideoAttachment) bool {
	for _, attachment := range attachments {
		if !s.HasAttachment(attachment.ID) {
			return false
		}
	}
	return true
}

func (s *State) RememberAttachment(creatorID, postID, attachmentID, filename string, released time.Time) {
	s.ensureMaps()
	s.Attachments[attachmentID] = AttachmentState{
		CreatorID:   creatorID,
		PostID:      postID,
		Filename:    filename,
		ReleaseDate: released.UTC().Format(time.RFC3339Nano),
	}
	s.RememberLatestRelease(creatorID, released)
}

func (s *State) RememberLatestRelease(creatorID string, released time.Time) {
	s.ensureMaps()
	current := s.LatestReleaseTime(creatorID)
	if !released.After(current) {
		return
	}
	s.Creators[creatorID] = CreatorState{
		LatestReleaseDate: released.UTC().Format(time.RFC3339Nano),
	}
}

func (s *State) LatestReleaseTime(creatorID string) time.Time {
	s.ensureMaps()
	creator, ok := s.Creators[creatorID]
	if !ok || creator.LatestReleaseDate == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, creator.LatestReleaseDate)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (s *State) AccessToken() string {
	s.ensureMaps()
	return s.Auth.AccessToken
}

func (s *State) RefreshToken() string {
	s.ensureMaps()
	return s.Auth.RefreshToken
}

func (s *State) TokenExpiresAt() time.Time {
	s.ensureMaps()
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
	s.ensureMaps()
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
	s.ensureMaps()
	s.Auth = AuthState{}
}

func (s *State) ensureMaps() {
	if s.Creators == nil {
		s.Creators = map[string]CreatorState{}
	}
	if s.Attachments == nil {
		s.Attachments = map[string]AttachmentState{}
	}
}
