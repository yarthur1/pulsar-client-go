// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package auth

import (
	"crypto/tls"
	"fmt"

	"github.com/yarthur1/pulsar-client-go/oauth2"
	"github.com/yarthur1/pulsar-client-go/oauth2/cache"
	"github.com/yarthur1/pulsar-client-go/oauth2/clock"
	"github.com/yarthur1/pulsar-client-go/oauth2/store"
)

const (
	ConfigParamType                  = "type"
	ConfigParamTypeClientCredentials = "client_credentials"
	ConfigParamIssuerURL             = "issuerUrl"
	ConfigParamAudience              = "audience"
	ConfigParamKeyFile               = "privateKey"
	ConfigParamClientID              = "clientId"
)

type oauth2AuthProvider struct {
	clock  clock.Clock
	issuer oauth2.Issuer
	store  store.Store
	source cache.CachingTokenSource
}

// NewAuthenticationOAuth2WithParams return a interface of Provider with string map.
func NewAuthenticationOAuth2WithParams(params map[string]string) (Provider, error) {
	issuer := oauth2.Issuer{
		IssuerEndpoint: params[ConfigParamIssuerURL],
		ClientID:       params[ConfigParamClientID],
		Audience:       params[ConfigParamAudience],
	}

	// initialize a store of authorization grants
	st := store.NewMemoryStore()
	switch params[ConfigParamType] {
	case ConfigParamTypeClientCredentials:
		flow, err := oauth2.NewDefaultClientCredentialsFlow(oauth2.ClientCredentialsFlowOptions{
			KeyFile:          params[ConfigParamKeyFile],
			AdditionalScopes: nil,
		})
		if err != nil {
			return nil, err
		}
		grant, err := flow.Authorize(issuer.Audience)
		if err != nil {
			return nil, err
		}
		err = st.SaveGrant(issuer.Audience, *grant)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported authentication type: %s", params[ConfigParamType])
	}

	return NewAuthenticationOAuth2(issuer, st), nil
}

func NewAuthenticationOAuth2(
	issuer oauth2.Issuer,
	store store.Store) Provider {

	return &oauth2AuthProvider{
		clock:  clock.RealClock{},
		issuer: issuer,
		store:  store,
	}
}

func (p *oauth2AuthProvider) Init() error {
	grant, err := p.store.LoadGrant(p.issuer.Audience)
	if err != nil {
		if err == store.ErrNoAuthenticationData {
			return nil
		}
		return err
	}
	refresher, err := p.getRefresher(grant.Type)
	if err != nil {
		return err
	}

	source, err := cache.NewDefaultTokenCache(p.store, p.issuer.Audience, refresher)
	if err != nil {
		return err
	}
	p.source = source
	return nil
}

func (p *oauth2AuthProvider) Name() string {
	return "token"
}

func (p *oauth2AuthProvider) GetTLSCertificate() (*tls.Certificate, error) {
	return nil, nil
}

func (p *oauth2AuthProvider) GetData() ([]byte, error) {
	if p.source == nil {
		// anonymous access
		return nil, nil
	}
	token, err := p.source.Token()
	if err != nil {
		return nil, err
	}
	return []byte(token.AccessToken), nil
}

func (p *oauth2AuthProvider) Close() error {
	return nil
}

func (p *oauth2AuthProvider) getRefresher(t oauth2.AuthorizationGrantType) (oauth2.AuthorizationGrantRefresher, error) {
	switch t {
	case oauth2.GrantTypeClientCredentials:
		return oauth2.NewDefaultClientCredentialsGrantRefresher(p.clock)
	case oauth2.GrantTypeDeviceCode:
		return oauth2.NewDefaultDeviceAuthorizationGrantRefresher(p.clock)
	default:
		return nil, store.ErrUnsupportedAuthData
	}
}
