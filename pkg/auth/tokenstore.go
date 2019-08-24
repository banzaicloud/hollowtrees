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
	bauth "github.com/banzaicloud/bank-vaults/pkg/sdk/auth"
)

func NewNullTokenStore() bauth.TokenStore {
	return &nullTokenStore{}
}

type nullTokenStore struct {
}

func (tokenStore *nullTokenStore) Store(userID string, token *bauth.Token) error {
	return nil
}

func (tokenStore *nullTokenStore) Lookup(userID, tokenID string) (*bauth.Token, error) {
	return &bauth.Token{}, nil
}

func (tokenStore *nullTokenStore) Revoke(userID, tokenID string) error {
	return nil
}

func (tokenStore *nullTokenStore) List(userID string) ([]*bauth.Token, error) {
	return nil, nil
}

func (tokenStore *nullTokenStore) GC() error {
	return nil
}
