package juju

import (
	"fmt"
	"strings"

	"github.com/juju/juju/api/client/applicationoffers"
	"github.com/juju/juju/core/crossmodel"
	"github.com/juju/juju/rpc/params"
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
