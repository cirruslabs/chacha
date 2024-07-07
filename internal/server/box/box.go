package box

import (
	cryptorand "crypto/rand"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

const validityDuration = 5 * time.Minute

type Manager struct {
	key []byte
}

type Box struct {
	CacheKeyPrefix string `json:"ckp,omitempty"`
}

type claims struct {
	Box

	jwt.RegisteredClaims
}

func NewManager() (*Manager, error) {
	key := make([]byte, 32)

	_, err := cryptorand.Read(key)
	if err != nil {
		return nil, err
	}

	return &Manager{
		key: key,
	}, nil
}

func (manager *Manager) Seal(box Box) (string, error) {
	now := time.Now()

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Box: box,
		RegisteredClaims: jwt.RegisteredClaims{
			NotBefore: jwt.NewNumericDate(now.Add(-validityDuration)),
			ExpiresAt: jwt.NewNumericDate(now.Add(validityDuration)),
		},
	})

	return jwtToken.SignedString(manager.key)
}

func (manager *Manager) Unseal(sealedBox string) (Box, error) {
	var claims claims

	validMethods := []string{
		jwt.SigningMethodHS256.Alg(),
	}

	_, err := jwt.ParseWithClaims(sealedBox, &claims, manager.keyFunc, jwt.WithValidMethods(validMethods))
	if err != nil {
		return Box{}, err
	}

	return claims.Box, nil
}

func (manager *Manager) keyFunc(_ *jwt.Token) (interface{}, error) {
	return manager.key, nil
}
