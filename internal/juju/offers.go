package juju

type offersClient struct {
	ConnectionFactory
}

func newOffersClient(cf ConnectionFactory) *offersClient {
	return &offersClient{
		ConnectionFactory: cf,
	}
}
