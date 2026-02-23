package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/ports"
)

var ErrUnauthorized = errors.New("unauthorized")

type AuthService struct {
	repo ports.APIKeyRepository
}

func NewAuthService(repo ports.APIKeyRepository) *AuthService {
	return &AuthService{repo: repo}
}

func (s *AuthService) Authenticate(ctx context.Context, token string) (domain.APIKey, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.APIKey{}, ErrUnauthorized
	}

	hash := HashToken(token)
	apiKey, err := s.repo.FindByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.APIKey{}, ErrUnauthorized
		}
		return domain.APIKey{}, err
	}
	if !apiKey.Active {
		return domain.APIKey{}, ErrUnauthorized
	}
	return apiKey, nil
}

func HashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
