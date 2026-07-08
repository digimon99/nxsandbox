package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Config struct {
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	OTPValidWindow  time.Duration
	AdminEmails     []string
	CookieDomain    string
}

type Claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Role  string `json:"role"`
}

type OTPMailer interface {
	SendOTP(ctx context.Context, email, code string) error
}

type NoopMailer struct{}

func (n NoopMailer) SendOTP(ctx context.Context, email, code string) error {
	return nil
}

type Service struct {
	store  *Store
	mailer OTPMailer
	cfg    Config
}

func NewService(store *Store, mailer OTPMailer, cfg Config) *Service {
	if mailer == nil {
		mailer = NoopMailer{}
	}
	return &Service{store: store, mailer: mailer, cfg: cfg}
}

func (s *Service) RequestOTP(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return fmt.Errorf("email is required")
	}
	code, err := s.store.CreateOTP(ctx, email, s.cfg.OTPValidWindow)
	if err != nil {
		return err
	}
	return s.mailer.SendOTP(ctx, email, code)
}

func (s *Service) VerifyOTP(ctx context.Context, email, otp string) (string, string, *User, error) {
	if err := s.store.VerifyOTP(ctx, email, otp); err != nil {
		return "", "", nil, err
	}
	user, err := s.store.UpsertUser(ctx, email, s.cfg.AdminEmails)
	if err != nil {
		return "", "", nil, err
	}
	access, err := s.issueAccessToken(user)
	if err != nil {
		return "", "", nil, err
	}
	refresh, err := s.store.CreateRefreshToken(ctx, user.ID, s.cfg.RefreshTokenTTL)
	if err != nil {
		return "", "", nil, err
	}
	return access, refresh, user, nil
}

func (s *Service) RefreshSession(ctx context.Context, refresh string) (string, *User, error) {
	user, err := s.store.ValidateRefreshToken(ctx, refresh)
	if err != nil {
		return "", nil, err
	}
	access, err := s.issueAccessToken(user)
	if err != nil {
		return "", nil, err
	}
	return access, user, nil
}

func (s *Service) RevokeSession(ctx context.Context, refresh string) error {
	return s.store.RevokeRefreshToken(ctx, refresh)
}

func (s *Service) ValidateAccessToken(tokenStr string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func (s *Service) AccessTokenTTLSeconds() int64 {
	return int64(s.cfg.AccessTokenTTL.Seconds())
}

func (s *Service) RefreshTokenTTLSeconds() int {
	return int(s.cfg.RefreshTokenTTL.Seconds())
}

func (s *Service) issueAccessToken(user *User) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTokenTTL)),
			Issuer:    "nxsandbox",
		},
		Email: user.Email,
		Role:  user.Role,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(s.cfg.JWTSecret))
}
