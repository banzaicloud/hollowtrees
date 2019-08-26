// Copyright © 2019 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"

	bauth "github.com/banzaicloud/bank-vaults/pkg/sdk/auth"
)

type User struct {
	ClusterID string `json:"clusterID"`
	OrgID     string `json:"orgID"`
}

type TokenGenerator interface {
	Generate(userID, orgID uint, expiresAt *time.Time) (string, string, error)
}

type tokenGenerator struct {
	Issuer     string
	Audience   string
	SigningKey string
}

func NewTokenGenerator(issuer, audience, signingKey string) TokenGenerator {
	return &tokenGenerator{
		Issuer:     issuer,
		Audience:   audience,
		SigningKey: signingKey,
	}
}

func (g *tokenGenerator) Generate(userID, orgID uint, expiresAt *time.Time) (string, string, error) {
	tokenID := uuid.Must(uuid.NewV4()).String()

	var expiresAtUnix int64
	if expiresAt != nil {
		expiresAtUnix = expiresAt.Unix()
	}

	// Create the Claims
	claims := &bauth.ScopedClaims{
		StandardClaims: jwt.StandardClaims{
			Issuer:    g.Issuer,
			Audience:  g.Audience,
			IssuedAt:  jwt.TimeFunc().Unix(),
			ExpiresAt: expiresAtUnix,
			Subject:   fmt.Sprintf("clusters/%d/%d", orgID, userID),
			Id:        tokenID,
		},
		Scope: "api:invoke",
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	if g.SigningKey == "" {
		return "", "", errors.New("missing signingKeyBase32")
	}
	signedToken, err := jwtToken.SignedString([]byte(base32.StdEncoding.EncodeToString([]byte(g.SigningKey))))
	if err != nil {
		return "", "", errors.Wrap(err, "failed to sign user token")
	}

	return tokenID, signedToken, nil
}

func GetCurrentUser(c *gin.Context) *User {
	if u, ok := bauth.GetCurrentUser(c).(*User); ok {
		return u
	}
	return nil
}

func Handler(signingKey string) gin.HandlerFunc {
	return bauth.JWTAuth(nil, signingKey, claimConverter)
}

func claimConverter(claims *bauth.ScopedClaims) interface{} {
	if !strings.HasPrefix(claims.Subject, "clusters/") {
		return nil
	}

	segments := strings.Split(claims.Subject, "/")
	if len(segments) < 2 {
		return nil
	}

	return &User{
		ClusterID: segments[2],
		OrgID:     segments[1],
	}
}
