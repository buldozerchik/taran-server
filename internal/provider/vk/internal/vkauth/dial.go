package vkauth

import (
	"github.com/buldozerchik/taran-server/internal/provider/vk/internal/browserprofile"
	"github.com/buldozerchik/taran-server/internal/provider/vk/internal/personanet"

	tlsclient "github.com/bogdanfinn/tls-client"
)

func (c *Client) newTLSClient(profile browserprofile.Profile, jar tlsclient.CookieJar) (tlsclient.HttpClient, error) {
	return personanet.NewClient(profile, c.dialer, jar)
}
