package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type option func(*Claims)

type Claims struct {
	UID int64 `json:"uid"`
	jwt.RegisteredClaims
}

func (c *Claims) With(secrete []byte, opts ...option) (string, error) {

	for _, opt := range opts {
		opt(c)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(secrete)
}

// New can get a token
// you should set the uid
// or it will be set with current time
func New(secrete []byte, opts ...option) (string, error) {
	claim := Claims{
		UID: time.Now().Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "auth",
			Subject:   "login",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24 * 3)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return claim.With(secrete, opts...)
}

func WithExpireAt(expireAt time.Time) option {
	return func(c *Claims) {
		c.ExpiresAt = jwt.NewNumericDate(expireAt)
	}
}
func WithUID(uid int64) option {
	return func(c *Claims) {
		c.UID = uid
	}
}
