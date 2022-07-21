package juju

import (
	"fmt"
	"strings"

	apiapplication "github.com/juju/juju/api/client/application"
	"github.com/juju/juju/api/client/applicationoffers"
	apiclient "github.com/juju/juju/api/client/client"
	"github.com/juju/juju/core/crossmodel"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
)

type offersClient struct {
	ConnectionFactory
}

type CreateOfferInput struct {
	ApplicationName string
	Endpoint        string
	ModelName       string
	ModelUUID       string
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

func newOffersClient(cf ConnectionFactory) *offersClient {
	return &offersClient{
		ConnectionFactory: cf,
	}
}

func (c offersClient) CreateOffer(input *CreateOfferInput) (*CreateOfferResponse, []error) {
	var errs []error

	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, append(errs, err)
	}

	client := applicationoffers.NewClient(conn)
	defer client.Close()

	offerName := input.Name
	if offerName == "" {
		offerName = input.ApplicationName
	}

	result, err := client.Offer(input.ModelUUID, input.ApplicationName, []string{input.Endpoint}, "admin", offerName, "")
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

	client := applicationoffers.NewClient(conn)
	defer client.Close()

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

	client := applicationoffers.NewClient(conn)
	defer client.Close()

	//TODO: verify destruction after attaching
	forceDestroy := false
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

//This function allows the integration resource to consume the offers managed by the offer resource
func (c offersClient) ConsumeRemoteOffer(input *ConsumeRemoteOfferInput) (*ConsumeRemoteOfferResponse, error) {
	modelConn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	offersClient := applicationoffers.NewClient(conn)
	defer offersClient.Close()
	client := apiapplication.NewClient(modelConn)
	defer client.Close()

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
		return nil, err
	}

	response := ConsumeRemoteOfferResponse{
		SAASName: localName,
	}

	return &response, nil
}

//This function allows the integration resource to destroy the offers managed by the offer resource
func (c offersClient) RemoveRemoteOffer(input *RemoveRemoteOfferInput) []error {
	var errors []error
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		errors = append(errors, err)
		return errors
	}

	client := apiapplication.NewClient(conn)
	defer client.Close()
	clientAPIClient := apiclient.NewClient(conn)
	defer clientAPIClient.Close()

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
