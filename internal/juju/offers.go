// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	stderr "errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/juju/juju/api"
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

// RemoteAppNotFoundError is returned when a remote app
// cannot be found when contacting the Juju API.
var RemoteAppNotFoundError = errors.ConstError("remote-app-not-found")

type offersClient struct {
	SharedClient
}

// CreateOfferInput represents input for creating an offer.
type CreateOfferInput struct {
	ApplicationName string
	Endpoints       []string
	ModelUUID       string
	OfferOwner      string
	Name            string
}

// CreateOfferResponse represents the response from creating an offer.
type CreateOfferResponse struct {
	Name     string
	OfferURL string
}

// ReadOfferInput represents input for reading an offer.
type ReadOfferInput struct {
	OfferURL           string
	OfferingController string
	// GetModelUUID, if set, will populate the ModelUUID field in the response.
	// Only set this if you know the user has at least read access to
	// the model. E.g. if you are creating the offer you can be sure
	// that the user has read access to the model.
	GetModelUUID bool
}

// ReadOfferResponse represents the response from reading an offer.
type ReadOfferResponse struct {
	ApplicationName string
	Endpoints       []string
	ModelUUID       string
	Name            string
	OfferURL        string
	Users           []crossmodel.OfferUserDetails
}

// DestroyOfferInput represents input for destroying an offer.
type DestroyOfferInput struct {
	OfferURL string
}

// ConsumeRemoteOfferInput represents input for consuming a remote offer.
type ConsumeRemoteOfferInput struct {
	ModelUUID          string
	OfferURL           string
	RemoteAppAlias     string
	OfferingController string
}

// ConsumeRemoteOfferResponse represents the response from consuming a remote offer.
type ConsumeRemoteOfferResponse struct {
	SAASName string
}

// ReadRemoteAppInput represents input for reading a remote app.
type ReadRemoteAppInput struct {
	ModelUUID     string
	RemoteAppName string
}

// ReadRemoteAppResponse represents the response from reading a remote app.
type ReadRemoteAppResponse struct {
}

// RemoveRemoteAppInput represents input for removing a remote app.
type RemoveRemoteAppInput struct {
	ModelUUID     string
	RemoteAppName string
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

	modelOwner, modelName, err := c.ModelOwnerAndName(input.ModelUUID)
	if err != nil {
		return nil, append(errs, fmt.Errorf("unable to get model name for model UUID %q: %w", input.ModelUUID, err))
	}

	c.JujuLogger().sc.Debugf(fmt.Sprintf("listing offers to find the created offer %q in model %q owned by %q", offerName, modelName, modelOwner))

	filter := crossmodel.ApplicationOfferFilter{
		OfferName: offerName,
		ModelName: modelName,
		OwnerName: modelOwner,
	}

	offer, err := findCreatedOffer(client, filter, input.Endpoints, offerName)
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
	var conn api.Connection
	var err error
	if input.OfferingController != "" {
		conn, err = c.GetOfferingControllerConn(input.OfferingController)
	} else {
		conn, err = c.GetConnection(nil)
	}
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
		response.ModelUUID, err = c.ModelUUID(resultURL.ModelName, resultURL.User)
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
		c.Tracef(fmt.Sprintf("offer %q has %d connections, waiting for them to be removed before destroying", offer.OfferURL, len(offer.Connections)))
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

// matchByEndpoints is returning offers that match exactly the endpoints' names provided.
// If no endpoints are provided, all offers are returned to match the API behaviour.
// The reason why we rely on this custom matching and not the API filtering is that
// the API filtering doesn't work as expected when multiple endpoints filters are provided.
// An endpoint filter is composed of three fields: Name, Interface and Role.
// If we try to filter by two endpoints' names, the API will return no offers, because the internal
// logic is doing an AND on the different fields of the endpoints filter. Specifying two fields with the same
// field (ex. Name) will always result in no matches.
func matchByEndpoints(offers []*crossmodel.ApplicationOfferDetails, endpoints []string) []*crossmodel.ApplicationOfferDetails {
	if len(endpoints) == 0 {
		return offers
	}
	slices.Sort(endpoints)

	filtered := []*crossmodel.ApplicationOfferDetails{}
	for _, offer := range offers {
		if len(offer.Endpoints) != len(endpoints) {
			continue
		}
		endpointsNames := make([]string, 0, len(offer.Endpoints))
		for _, endpoint := range offer.Endpoints {
			endpointsNames = append(endpointsNames, endpoint.Name)
		}
		slices.Sort(endpointsNames)

		if slices.Equal(endpointsNames, endpoints) {
			filtered = append(filtered, offer)
		}
	}
	return filtered
}

// matchByOfferName is returning offers that match exactly the offer name provided.
//
// The FindApplicationOffers API doesn't support exact matching on offer name, it does a fuzzy match using
// regex ".*<offer-name>.*".
//
// If we create an offer called "haproxy-two", and then an offer called "haproxy", Juju will return both
// offers because it will covert the offer name of "haproxy" into a fuzzy match of ".*haproxy.*".
// To workaround this, we need to filter the offers by exact offer name.
//
// Secondly, this fuzzy match is across the model, so if there was another offer for a completely different
// application, this would match too.
//
// We do not need to filter by application though, as exact offer name match is enough to distinguish offers,
// even if there are multiple offers for the same/different application.
func matchByOfferName(offers []*crossmodel.ApplicationOfferDetails, offerName string) []*crossmodel.ApplicationOfferDetails {
	exactMatches := []*crossmodel.ApplicationOfferDetails{}
	for _, off := range offers {
		if off.OfferName == offerName {
			exactMatches = append(exactMatches, off)
		}
	}
	return exactMatches
}

// findCreatedOffer is a helper function to find the created offer after calling the Offer API.
//
// This is required because the Offer API doesn't return the offer URL of the created offer, or
// any kind of identifier to help find the offer.
func findCreatedOffer(client *applicationoffers.Client, filter crossmodel.ApplicationOfferFilter, endpoints []string, offerName string) (*crossmodel.ApplicationOfferDetails, error) {
	offers, err := client.FindApplicationOffers(filter)
	if err != nil {
		return nil, err
	}

	offers = matchByEndpoints(offers, endpoints)
	offers = matchByOfferName(offers, offerName)

	if len(offers) == 0 {
		return nil, fmt.Errorf("unable to find offer after creation")
	}

	if len(offers) > 1 {
		return nil, fmt.Errorf("%d offers found using filter after creation", len(offers))
	}

	return offers[0], nil
}

// ConsumeRemoteOffer allows the integration resource to consume the offers managed by the offer resource.
func (c offersClient) ConsumeRemoteOffer(input *ConsumeRemoteOfferInput) (*ConsumeRemoteOfferResponse, error) {
	if input.ModelUUID == "" {
		return nil, fmt.Errorf("missing model when attemtpting to consume an offer")
	}
	if input.OfferURL == "" {
		return nil, fmt.Errorf("missing offer URL when attempting to consume an offer")
	}
	// input.RemoteAppAlias can be empty to use the default offer name.

	url, err := crossmodel.ParseOfferURL(input.OfferURL)
	if err != nil {
		return nil, err
	}
	modelConn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = modelConn.Close() }()
	var conn api.Connection
	if input.OfferingController != "" {
		conn, err = c.GetOfferingControllerConn(input.OfferingController)
	} else {
		conn, err = c.GetConnection(nil)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	offeringControllerClient := applicationoffers.NewClient(conn)
	consumingClient := apiapplication.NewClient(modelConn)

	if url.HasEndpoint() {
		return nil, fmt.Errorf("saas offer %q shouldn't include endpoint", input.OfferURL)
	}
	consumeDetails, err := offeringControllerClient.GetConsumeDetails(url.AsLocal().String())
	if err != nil {
		return nil, err
	}

	offerURL, err := crossmodel.ParseOfferURL(consumeDetails.Offer.OfferURL)
	if err != nil {
		return nil, err
	}
	if input.OfferingController != "" {
		offerURL.Source = input.OfferingController
	}
	consumeDetails.Offer.OfferURL = offerURL.String()

	consumeArgs := crossmodel.ConsumeApplicationArgs{
		Offer:            *consumeDetails.Offer,
		ApplicationAlias: input.RemoteAppAlias,
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

	localName, err := consumingClient.Consume(consumeArgs)
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

// ReadRemoteApp allows for reading details of a consumed offer
// i.e. reading a SAAS (remote-app).
//
// The naming is confusing here as the `juju status --format yaml` output shows
// these objects under "application-endpoints", the API calls them RemoteApplications
// and `juju status` shows them under the "SAAS" heading.
func (c offersClient) ReadRemoteApp(input *ReadRemoteAppInput) (*ReadRemoteAppResponse, error) {
	modelConn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = modelConn.Close() }()

	clientAPIClient := apiclient.NewClient(modelConn, c.JujuLogger())

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch model status: %w", err)
	}

	remoteApplications := status.RemoteApplications

	if len(remoteApplications) == 0 {
		return nil, errors.WithType(errors.New("remote app not found"), RemoteAppNotFoundError)
	}

	for appName := range remoteApplications {
		if appName == input.RemoteAppName {
			return &ReadRemoteAppResponse{}, nil
		}
	}

	return nil, errors.WithType(errors.New("remote app not found"), RemoteAppNotFoundError)
}

// RemoveRemoteApp allows the integration resource to destroy the offers managed by the offer resource.
func (c offersClient) RemoveRemoteApp(input *RemoveRemoteAppInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := apiapplication.NewClient(conn)
	clientAPIClient := apiclient.NewClient(conn, c.JujuLogger())

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		return err
	}

	remoteApplications := status.RemoteApplications

	if len(remoteApplications) == 0 {
		return fmt.Errorf("no offers found in model")
	}

	var offerName string
	for appName := range remoteApplications {
		if appName == input.RemoteAppName {
			offerName = appName
			break
		}
	}

	if offerName == "" {
		return fmt.Errorf("remote-app %q not found in model", input.RemoteAppName)
	}

	// This is a bulk call but we only want to remove one remote app
	// so we expect only a single error to be returned if it fails.
	returnErrors, err := client.DestroyConsumedApplication(apiapplication.DestroyConsumedApplicationParams{
		SaasNames: []string{
			offerName,
		},
	})
	if err != nil {
		return err
	}

	var errors []error
	for _, v := range returnErrors {
		if v.Error != nil {
			errors = append(errors, v.Error)
		}
	}
	return stderr.Join(errors...)
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
