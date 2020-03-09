package provider

import (
	"github.com/aaronland/go-artisanal-integers"
	"github.com/whosonfirst/go-whosonfirst-id"
)

type ProxyServiceProvider struct {
	id.Provider
	proxy_service artisanalinteger.Service
}

func NewProxyServiceProvider(service artisanalinteger.Service) (id.Provider, error) {

	pr := &ProxyServiceProvider{
		proxy_service: service,
	}

	return pr, nil
}

func (pr *ProxyServiceProvider) NewID() (int64, error) {
	return pr.proxy_service.NextInt()
}
