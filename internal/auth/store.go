package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrRateLimited = errors.New("too many OTP requests, please retry later")
	ErrInvalidOTP  = errors.New("invalid or expired verification code")
	ErrInvalidRT   = errors.New("invalid or expired refresh token")
)

type User struct {
	ID         string
	Email      string
	Role       string
	CreatedAt  time.Time
	LastSignin *time.Time
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateOTP(ctx context.Context, email string, ttl time.Duration) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	var count int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM otp_codes
		WHERE email = $1
		  AND created_at > now() - INTERVAL '15 minutes'
		  AND expires_at > now()
	`, email).Scan(&count); err != nil {
		return "", err
	}
	if count >= 5 {
		return "", ErrRateLimited
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	hashed, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO otp_codes(email, hash, expires_at)
		VALUES ($1, $2, $3)
	`, email, string(hashed), time.Now().Add(ttl))
	if err != nil {
		return "", err
	}

	return code, nil
}

func (s *Store) VerifyOTP(ctx context.Context, email, code string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)

	var id string
	var hash string
	var attempts int
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, hash, attempts
		FROM otp_codes
		WHERE email = $1
		  AND used = false
		  AND expires_at > now()
		ORDER BY created_at DESC
		LIMIT 1
	`, email).Scan(&id, &hash, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidOTP
	}
	if err != nil {
		return err
	}

	if attempts >= 5 {
		return ErrInvalidOTP
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)); err != nil {
		_, _ = s.pool.Exec(ctx, `UPDATE otp_codes SET attempts = attempts + 1 WHERE id = $1::uuid`, id)
		return ErrInvalidOTP
	}

	_, err = s.pool.Exec(ctx, `UPDATE otp_codes SET used = true WHERE id = $1::uuid`, id)
	return err
}

func (s *Store) UpsertUser(ctx context.Context, email string, adminEmails []string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	role := "user"
	for _, e := range adminEmails {
		if strings.EqualFold(strings.TrimSpace(e), email) {
			role = "admin"
			break
		}
	}

	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users(email, role, last_signin, updated_at)
		VALUES ($1, $2, now(), now())
		ON CONFLICT (email) DO UPDATE SET
			last_signin = now(),
			updated_at = now(),
			role = CASE WHEN users.role = 'admin' THEN 'admin' ELSE EXCLUDED.role END
		RETURNING id::text, email, role, created_at, last_signin
	`, email, role).Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt, &u.LastSignin)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateRefreshToken(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	_, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens(user_id, hash, expires_at)
		VALUES ($1::uuid, $2, $3)
	`, userID, hash, time.Now().Add(ttl))
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *Store) ValidateRefreshToken(ctx context.Context, token string) (*User, error) {
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.role, u.created_at, u.last_signin
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.hash = $1
		  AND rt.revoked = false
		  AND rt.expires_at > now()
	`, hash).Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt, &u.LastSignin)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidRT
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) RevokeRefreshToken(ctx context.Context, token string) error {
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])
	_, err := s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked = true WHERE hash = $1`, hash)
	return err
}
