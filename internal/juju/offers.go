// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apiapplication "github.com/juju/juju/api/client/application"
	"github.com/juju/juju/api/client/applicationoffers"
	apiclient "github.com/juju/juju/api/client/client"
	"github.com/juju/juju/core/crossmodel"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v5"
)

const (
	// OfferAppAvailableTimeout is the time to wait for an app to be available
	// before creating an offer.
	OfferAppAvailableTimeout = time.Second * 60
	// OfferApiTickWait is the time to wait between consecutive requests
	// to the API
	OfferApiTickWait = time.Second * 5
)

type offersClient struct {
	SharedClient
}

type CreateOfferInput struct {
	ApplicationName string
	Endpoints       []string
	ModelUUID       string
	OfferOwner      string
	Name            string
}

type CreateOfferResponse struct {
	Name     string
	OfferURL string
}

type ReadOfferInput struct {
	OfferURL string
	// GetModelUUID, if set, will populate the ModelUUID field in the response.
	// Only set this if you know the user has at least read access to
	// the model. E.g. if you are creating the offer you can be sure
	// that the user has read access to the model.
	GetModelUUID bool
}

type ReadOfferResponse struct {
	ApplicationName string
	Endpoints       []string
	ModelUUID       string
	Name            string
	OfferURL        string
	Users           []crossmodel.OfferUserDetails
}

type DestroyOfferInput struct {
	OfferURL string
}

type ConsumeRemoteOfferInput struct {
	ModelUUID string
	OfferURL  string
}

type ConsumeRemoteOfferResponse struct {
	SAASName string
}

type RemoveRemoteOfferInput struct {
	ModelUUID string
	OfferURL  string
}

// GrantRevokeOfferInput represents input for granting or revoking access to an offer.
type GrantRevokeOfferInput struct {
	Users    []string
	Access   string
	OfferURL string
}

func newOffersClient(sc SharedClient) *offersClient {
	return &offersClient{
		SharedClient: sc,
	}
}

// CreateOffer creates offer managed by the offer resource.
func (c offersClient) CreateOffer(input *CreateOfferInput) (*CreateOfferResponse, []error) {
	var errs []error

	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, append(errs, err)
	}
	defer func() { _ = conn.Close() }()

	client := applicationoffers.NewClient(conn)

	offerName := input.Name
	if offerName == "" {
		offerName = input.ApplicationName
	}

	// connect to the corresponding model
	modelConn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, append(errs, err)
	}
	defer func() { _ = modelConn.Close() }()
	applicationClient := apiapplication.NewClient(modelConn)

	// wait for the app to be available
	ctx, cancel := context.WithTimeout(context.Background(), OfferAppAvailableTimeout)
	defer cancel()

	err = WaitForAppsAvailable(ctx, applicationClient, []string{input.ApplicationName}, OfferApiTickWait)
	if err != nil {
		return nil, append(errs, errors.New("the application was not available to be offered"))
	}

	result, err := client.Offer(input.ModelUUID, input.ApplicationName, input.Endpoints, input.OfferOwner, offerName, "")
	if err != nil {
		return nil, append(errs, err)
	}

	for _, v := range result {
		var result = params.ErrorResult{}
		if v == result {
			continue
		} else {
			errs = append(errs, v.Error)
		}
	}
	if len(errs) != 0 {
		return nil, errs
	}

	modelOwner, modelName, err := c.SharedClient.ModelOwnerAndName(input.ModelUUID)
	if err != nil {
		return nil, append(errs, fmt.Errorf("unable to get model name for model UUID %q: %w", input.ModelUUID, err))
	}

	filter := crossmodel.ApplicationOfferFilter{
		OfferName: offerName,
		ModelName: modelName,
		OwnerName: modelOwner,
	}

	offer, err := findApplicationOffers(client, filter)
	if err != nil {
		return nil, append(errs, err)
	}

	resp := CreateOfferResponse{
		Name:     offerName,
		OfferURL: offer.OfferURL,
	}
	return &resp, nil
}

// ReadOffer reads offer managed by the offer resource.
func (c offersClient) ReadOffer(input *ReadOfferInput) (*ReadOfferResponse, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := applicationoffers.NewClient(conn)
	result, err := client.ApplicationOffer(input.OfferURL)
	if err != nil {
		return nil, err
	}

	resultURL, err := crossmodel.ParseOfferURL(result.OfferURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse offer URL %q: %w", result.OfferURL, err)
	}
	resultURL.Source = "" // Ensure the source is empty for consistency

	// If the ID used to query the resource does not match the one in the result, it can result
	// in an unexpected value when saved to a resource, so fail early for easier diagnostics.
	if resultURL.String() != input.OfferURL {
		return nil, fmt.Errorf("offer URL %q does not match the expected URL %q", result.OfferURL, input.OfferURL)
	}

	var response ReadOfferResponse
	response.Name = result.OfferName
	response.ApplicationName = result.ApplicationName
	response.OfferURL = resultURL.String()
	for _, endpoint := range result.Endpoints {
		response.Endpoints = append(response.Endpoints, endpoint.Name)
	}
	response.Users = result.Users

	if input.GetModelUUID {
		// TODO(JUJU-8299): The modelUUID method needs to be changed to also use the model owner.
		// Do this after all resources reference models by UUID and we can clean up the model cache.
		response.ModelUUID, err = c.ModelUUID(resultURL.ModelName)
		if err != nil {
			return nil, fmt.Errorf("unable to get model UUID for model %q: %w", resultURL.ModelName, err)
		}
	}

	return &response, nil
}

// DestroyOffer destroys offer managed by the offer resource.
func (c offersClient) DestroyOffer(input *DestroyOfferInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := applicationoffers.NewClient(conn)
	offer, err := client.ApplicationOffer(input.OfferURL)
	if err != nil {
		return err
	}

	forceDestroy := false
	//This code loops until it detects 0 connections in the offer or 3 minutes elapses
	if len(offer.Connections) > 0 {
		end := time.Now().Add(5 * time.Minute)
		for ok := true; ok; ok = len(offer.Connections) > 0 {
			//if we have been failing to destroy offer for 5 minutes then force destroy
			//TODO: investigate cleaner solution (acceptance tests fail even if timeout set to 20m)
			if time.Now().After(end) {
				forceDestroy = true
				break
			}
			time.Sleep(10 * time.Second)
			offer, err = client.ApplicationOffer(input.OfferURL)
			if err != nil {
				return err
			}
		}
	}

	err = client.DestroyOffers(forceDestroy, input.OfferURL)
	if err != nil {
		return err
	}

	return nil
}

func findApplicationOffers(client *applicationoffers.Client, filter crossmodel.ApplicationOfferFilter) (*crossmodel.ApplicationOfferDetails, error) {
	offers, err := client.FindApplicationOffers(filter)
	if err != nil {
		return nil, err
	}

	if len(offers) > 1 || len(offers) == 0 {
		return nil, fmt.Errorf("unable to find offer after creation")
	}

	return offers[0], nil
}

// ConsumeRemoteOffer allows the integration resource to consume the offers managed by the offer resource.
func (c offersClient) ConsumeRemoteOffer(input *ConsumeRemoteOfferInput) (*ConsumeRemoteOfferResponse, error) {
	modelConn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = modelConn.Close() }()
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	offersClient := applicationoffers.NewClient(conn)
	client := apiapplication.NewClient(modelConn)

	url, err := crossmodel.ParseOfferURL(input.OfferURL)
	if err != nil {
		return nil, err
	}

	if url.HasEndpoint() {
		return nil, fmt.Errorf("saas offer %q shouldn't include endpoint", input.OfferURL)
	}

	consumeDetails, err := offersClient.GetConsumeDetails(url.AsLocal().String())
	if err != nil {
		return nil, err
	}

	offerURL, err := crossmodel.ParseOfferURL(consumeDetails.Offer.OfferURL)
	if err != nil {
		return nil, err
	}

	// The offer URL should not have a source, as that would indicate a cross-controller
	// relation which is not strictly supported. Support for cross-controller relations
	// would require some changes above to identify if the URL is pointing to a different
	// controller than the one we are currently connected to and fetch the consume details
	// from there instead.
	if offerURL.Source != "" {
		return nil, fmt.Errorf("offer URL %q should not have a source", consumeDetails.Offer.OfferURL)
	}

	consumeDetails.Offer.OfferURL = offerURL.String()

	consumeArgs := crossmodel.ConsumeApplicationArgs{
		Offer:            *consumeDetails.Offer,
		ApplicationAlias: consumeDetails.Offer.OfferName,
		Macaroon:         consumeDetails.Macaroon,
	}
	if consumeDetails.ControllerInfo != nil {
		controllerTag, err := names.ParseControllerTag(consumeDetails.ControllerInfo.ControllerTag)
		if err != nil {
			return nil, err
		}
		consumeArgs.ControllerInfo = &crossmodel.ControllerInfo{
			ControllerTag: controllerTag,
			Alias:         consumeDetails.ControllerInfo.Alias,
			Addrs:         consumeDetails.ControllerInfo.Addrs,
			CACert:        consumeDetails.ControllerInfo.CACert,
		}
	}

	localName, err := client.Consume(consumeArgs)
	if err != nil {
		// Check if SAAS is already created. If so return offer response instead of error
		// TODO: Understand why jujuerrors.AlreadyExists is not working and use
		// the same for below condition
		if strings.Contains(err.Error(), "saas application already exists") {
			/* The logic to populate localName is picked from on how the juju controller
			   derives localName in Consume request.
			   https://github.com/juju/juju/blob/3e561add5940a510f785c83076b2bcc6994db103/api/client/application/client.go#L803
			*/
			localName = consumeArgs.Offer.OfferName
			if consumeArgs.ApplicationAlias != "" {
				localName = consumeArgs.ApplicationAlias
			}
		} else {
			return nil, err
		}
	}

	response := ConsumeRemoteOfferResponse{
		SAASName: localName,
	}

	return &response, nil
}

// RemoveRemoteOffer allows the integration resource to destroy the offers managed by the offer resource.
func (c offersClient) RemoveRemoteOffer(input *RemoveRemoteOfferInput) []error {
	var errors []error
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		errors = append(errors, err)
		return errors
	}
	defer func() { _ = conn.Close() }()

	client := apiapplication.NewClient(conn)
	clientAPIClient := apiclient.NewClient(conn, c.JujuLogger())

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		errors = append(errors, err)
		return errors
	}

	remoteApplications := status.RemoteApplications

	if len(remoteApplications) == 0 {
		errors = append(errors, fmt.Errorf("no offers found in model"))
		return errors
	}

	var offerName string
	for _, v := range remoteApplications {
		if v.Err != nil {
			errors = append(errors, v.Err)
			return errors
		}
		if v.OfferURL != input.OfferURL {
			continue
		}
		offerName = v.OfferName
	}

	returnErrors, err := client.DestroyConsumedApplication(apiapplication.DestroyConsumedApplicationParams{
		SaasNames: []string{
			offerName,
		},
	})
	if err != nil {
		errors = append(errors, err)
		return errors
	}

	for _, v := range returnErrors {
		if v.Error != nil {
			errors = append(errors, v.Error)
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// GrantOffer adds access to an offer managed by the access offer resource.
// No action or error is returned if the access was already granted to the user.
func (c offersClient) GrantOffer(input *GrantRevokeOfferInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := applicationoffers.NewClient(conn)

	for _, user := range input.Users {
		err = client.GrantOffer(user, input.Access, input.OfferURL)
		if err != nil {
			// ignore if user was already granted
			if strings.Contains(err.Error(), "user already has") {
				continue
			}
			return err
		}
	}

	return nil
}

// RevokeOffer revokes access to an offer managed by the access offer resource.
// No action or error if the access was already revoked for the user.
// Note: revoking `ReadAccess` will remove all access levels for the offer
func (c offersClient) RevokeOffer(input *GrantRevokeOfferInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := applicationoffers.NewClient(conn)

	for _, user := range input.Users {
		err = client.RevokeOffer(user, input.Access, input.OfferURL)
		if err != nil {
			// ignore if user was already revoked
			if strings.Contains(err.Error(), "not found") {
				continue
			}
			return err
		}
	}

	return nil
}
