package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session expired")
	ErrAccountInactive    = errors.New("account is inactive")
)

type Session struct {
	ID        string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Store struct {
	db  *sql.DB
	ttl time.Duration
}

func NewStore(db *sql.DB, ttlHours int) *Store {
	return &Store{db: db, ttl: time.Duration(ttlHours) * time.Hour}
}

// HashPassword uses bcrypt with the default (safe) cost.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateSession issues a new opaque session token for userID and persists it.
func (s *Store) CreateSession(userID, userAgent, ip string) (*Session, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	expires := now.Add(s.ttl)

	_, err = s.db.Exec(
		`INSERT INTO sessions (id, user_id, created_at, expires_at, user_agent, ip_address) VALUES (?, ?, ?, ?, ?, ?)`,
		token, userID, now, expires, userAgent, ip,
	)
	if err != nil {
		return nil, err
	}

	return &Session{ID: token, UserID: userID, CreatedAt: now, ExpiresAt: expires}, nil
}

// ValidateSession looks up a token and returns the associated session if
// still valid, expiring (and deleting) it otherwise.
func (s *Store) ValidateSession(token string) (*Session, error) {
	row := s.db.QueryRow(`SELECT id, user_id, created_at, expires_at FROM sessions WHERE id = ?`, token)

	var sess Session
	var createdAt, expiresAt string
	if err := row.Scan(&sess.ID, &sess.UserID, &createdAt, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	sess.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	sess.ExpiresAt, _ = time.Parse(time.RFC3339, normalizeTime(expiresAt))

	if time.Now().UTC().After(sess.ExpiresAt) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE id = ?`, token)
		return nil, ErrSessionExpired
	}

	return &sess, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, token)
	return err
}

// DeleteAllUserSessions revokes every session for a user (used on password change).
func (s *Store) DeleteAllUserSessions(userID string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// normalizeTime handles SQLite's "YYYY-MM-DD HH:MM:SS" default format by
// converting it into RFC3339 so time.Parse succeeds regardless of driver
// formatting quirks.
func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}
