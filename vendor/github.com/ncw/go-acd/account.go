// Copyright (c) 2015 Serge Gebhardt. All rights reserved.
//
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package acd

import (
	"net/http"
	"net/url"
	"time"
)

// AccountService provides access to the account related functions
// in the Amazon Cloud Drive API.
//
// See: https://developer.amazon.com/public/apis/experience/cloud-drive/content/account
type AccountService struct {
	client *Client
}

// AccountEndpoints represents information about the current customer's endpoints
type AccountEndpoints struct {
	CustomerExists bool   `json:"customerExists"`
	ContentURL     string `json:"contentUrl"`
	MetadataURL    string `json:"metadataUrl"`
}

// GetEndpoints retrives the current endpoints for this customer
//
// It also updates the endpoints in the client
func (s *AccountService) GetEndpoints() (*AccountEndpoints, *http.Response, error) {
	req, err := s.client.NewMetadataRequest("GET", "account/endpoint", nil)
	if err != nil {
		return nil, nil, err
	}

	endpoints := &AccountEndpoints{}
	resp, err := s.client.Do(req, endpoints)
	if err != nil {
		return nil, resp, err
	}

	// Update the client endpoints
	if endpoints.MetadataURL != "" {
		u, err := url.Parse(endpoints.MetadataURL)
		if err == nil {
			s.client.MetadataURL = u
		}
	}
	if endpoints.ContentURL != "" {
		u, err := url.Parse(endpoints.ContentURL)
		if err == nil {
			s.client.ContentURL = u
		}
	}

	return endpoints, resp, err
}

// AccountInfo represents information about an Amazon Cloud Drive account.
type AccountInfo struct {
	TermsOfUse *string `json:"termsOfUse"`
	Status     *string `json:"status"`
}

// GetInfo provides information about the current user account like
// the status and the accepted “Terms Of Use”.
func (s *AccountService) GetInfo() (*AccountInfo, *http.Response, error) {
	req, err := s.client.NewMetadataRequest("GET", "account/info", nil)
	if err != nil {
		return nil, nil, err
	}

	accountInfo := &AccountInfo{}
	resp, err := s.client.Do(req, accountInfo)
	if err != nil {
		return nil, resp, err
	}

	return accountInfo, resp, err
}

// AccountQuota represents information about the account quotas.
type AccountQuota struct {
	Quota          *uint64    `json:"quota"`
	LastCalculated *time.Time `json:"lastCalculated"`
	Available      *uint64    `json:"available"`
}

// GetQuota gets account quota and storage availability information.
func (s *AccountService) GetQuota() (*AccountQuota, *http.Response, error) {
	req, err := s.client.NewMetadataRequest("GET", "account/quota", nil)
	if err != nil {
		return nil, nil, err
	}

	accountQuota := &AccountQuota{}
	resp, err := s.client.Do(req, accountQuota)
	if err != nil {
		return nil, resp, err
	}

	return accountQuota, resp, err
}

// AccountUsage represents information about the account usage.
type AccountUsage struct {
	LastCalculated *time.Time     `json:"lastCalculated"`
	Other          *CategoryUsage `json:"other"`
	Doc            *CategoryUsage `json:"doc"`
	Photo          *CategoryUsage `json:"photo"`
	Video          *CategoryUsage `json:"video"`
}

// CategoryUsage defines Total and Billable UsageNumbers
type CategoryUsage struct {
	Total    *UsageNumbers `json:"total"`
	Billable *UsageNumbers `json:"billable"`
}

// UsageNumbers defines Bytes and Count for a metered count
type UsageNumbers struct {
	Bytes *uint64 `json:"bytes"`
	Count *uint64 `json:"count"`
}

// GetUsage gets Account Usage information broken down by content category.
func (s *AccountService) GetUsage() (*AccountUsage, *http.Response, error) {
	req, err := s.client.NewMetadataRequest("GET", "account/usage", nil)
	if err != nil {
		return nil, nil, err
	}

	accountUsage := &AccountUsage{}
	resp, err := s.client.Do(req, accountUsage)
	if err != nil {
		return nil, resp, err
	}

	return accountUsage, resp, err
}
