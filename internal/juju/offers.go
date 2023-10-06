// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/juju/juju/api/client/application"
	apiapplication "github.com/juju/juju/api/client/application"
	"github.com/juju/juju/api/client/applicationoffers"
	apiclient "github.com/juju/juju/api/client/client"
	"github.com/juju/juju/core/crossmodel"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
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
	Endpoint        string
	ModelName       string
	ModelOwner      string
	Name            string
}

type CreateOfferResponse struct {
	Name     string
	OfferURL string
}

type ReadOfferInput struct {
	OfferURL string
}

type ReadOfferResponse struct {
	ApplicationName string
	Endpoint        string
	ModelName       string
	Name            string
	OfferURL        string
}

type DestroyOfferInput struct {
	OfferURL string
}

type ConsumeRemoteOfferInput struct {
	ModelName string
	OfferURL  string
}

type ConsumeRemoteOfferResponse struct {
	SAASName string
}

type RemoveRemoteOfferInput struct {
	ModelName string
	OfferURL  string
}

func newOffersClient(sc SharedClient) *offersClient {
	return &offersClient{
		SharedClient: sc,
	}
}

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
	modelConn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return nil, append(errs, err)
	}
	defer func() { _ = modelConn.Close() }()
	applicationClient := application.NewClient(modelConn)

	// wait for the app to be available
	ctx, cancel := context.WithTimeout(context.Background(), OfferAppAvailableTimeout)
	defer cancel()

	err = WaitForAppsAvailable(ctx, applicationClient, []string{input.ApplicationName}, OfferApiTickWait)
	if err != nil {
		return nil, append(errs, errors.New("the application was not available to be offered"))
	}

	modelUUID, err := c.ModelUUID(input.ModelName)
	if err != nil {
		return nil, append(errs, err)
	}
	result, err := client.Offer(modelUUID, input.ApplicationName, []string{input.Endpoint}, "admin", offerName, "")
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

	filter := crossmodel.ApplicationOfferFilter{
		OfferName: offerName,
		ModelName: input.ModelName,
		OwnerName: input.ModelOwner,
	}

	offer, err := findApplicationOffers(client, filter)
	if err != nil {
		return nil, append(errs, err)
	}

	resp := CreateOfferResponse{
		Name:     offer.OfferName,
		OfferURL: offer.OfferURL,
	}
	return &resp, nil
}

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

	var response ReadOfferResponse
	response.Name = result.OfferName
	response.ApplicationName = result.ApplicationName
	response.OfferURL = result.OfferURL
	response.Endpoint = result.Endpoints[0].Name

	//no model name is returned but it can be parsed from the resulting offer URL to ensure parity
	//TODO: verify if we can fetch information another way
	modelName, ok := parseModelFromURL(result.OfferURL)
	if !ok {
		return nil, fmt.Errorf("unable to parse model name from offer URL")
	}
	response.ModelName = modelName

	return &response, nil
}

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

func parseModelFromURL(url string) (result string, success bool) {
	start := strings.Index(url, "/")
	if start == -1 {
		return result, false
	}
	newURL := url[start+1:]
	end := strings.Index(newURL, ".")
	if end == -1 {
		return result, false
	}
	result = newURL[:end]
	return result, true
}

// This function allows the integration resource to consume the offers managed by the offer resource
func (c offersClient) ConsumeRemoteOffer(input *ConsumeRemoteOfferInput) (*ConsumeRemoteOfferResponse, error) {
	modelConn, err := c.GetConnection(&input.ModelName)
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
	offerURL.Source = url.Source
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

// This function allows the integration resource to destroy the offers managed by the offer resource
func (c offersClient) RemoveRemoteOffer(input *RemoveRemoteOfferInput) []error {
	var errors []error
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		errors = append(errors, err)
		return errors
	}
	defer func() { _ = conn.Close() }()

	client := apiapplication.NewClient(conn)
	clientAPIClient := apiclient.NewClient(conn)

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
