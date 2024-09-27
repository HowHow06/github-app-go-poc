package main

import (
	"fmt"
	"net/http"

	"github.com/kfcampbell/ghinstallation"
	kiotaHttp "github.com/microsoft/kiota-http-go"
	"github.com/octokit/go-sdk/pkg"
	auth "github.com/octokit/go-sdk/pkg/authentication"
	"github.com/octokit/go-sdk/pkg/github"
	"github.com/octokit/go-sdk/pkg/handlers"
)

type ClientOptions struct {
	pkg.ClientOptions
	GitHubAppPemValue string
}

// ClientOptionFunc provides a functional pattern for client configuration
type ClientOptionFunc func(*ClientOptions)

// WithGitHubAppAuthentication configures the client with the given GitHub App auth.
func WithGitHubAppAuthenticationUsingPrivateKeyValue(pemValue string, clientID string, installationID int64) ClientOptionFunc {
	return func(c *ClientOptions) {
		c.GitHubAppPemValue = pemValue
		c.GitHubAppClientID = clientID
		c.GitHubAppInstallationID = installationID
	}
}

// To convert a original ClientOptionFunc to the extended version
func ClientOptionFuncAdapter(original pkg.ClientOptionFunc) ClientOptionFunc {
	return func(options *ClientOptions) {
		original(&options.ClientOptions)
	}
}

// NewApiClient is the extended implementation of Octokit Github API Client
// so that it accepts PEM (private key) string value as its Github App authentication
// By default, it includes a rate limiting middleware.
func NewApiClient(optionFuncs ...ClientOptionFunc) (*pkg.Client, error) {
	options, err := pkg.GetDefaultClientOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to get default client options: %v", err)
	}

	extendedOptions := &ClientOptions{
		ClientOptions: *options,
	}

	for _, optionFunc := range optionFuncs {
		optionFunc(extendedOptions)
	}

	rateLimitHandler := handlers.NewRateLimitHandler()
	middlewares := extendedOptions.Middleware
	middlewares = append(middlewares, rateLimitHandler)
	defaultTransport := kiotaHttp.GetDefaultTransport()
	netHttpClient := &http.Client{
		Transport: defaultTransport,
	}

	if extendedOptions.RequestTimeout != 0 {
		netHttpClient.Timeout = extendedOptions.RequestTimeout
	}

	if (extendedOptions.GitHubAppID != 0 || extendedOptions.GitHubAppClientID != "") && extendedOptions.GitHubAppInstallationID != 0 && (extendedOptions.GitHubAppPemFilePath != "" || extendedOptions.GitHubAppPemValue != "") {
		existingTransport := netHttpClient.Transport
		var appTransport *ghinstallation.Transport
		var err error

		if extendedOptions.GitHubAppPemFilePath != "" {
			if extendedOptions.GitHubAppClientID != "" {
				appTransport, err = ghinstallation.NewKeyFromFile(existingTransport, extendedOptions.GitHubAppClientID, extendedOptions.GitHubAppInstallationID, extendedOptions.GitHubAppPemFilePath)
			} else {
				appTransport, err = ghinstallation.NewKeyFromFileWithAppID(existingTransport, extendedOptions.GitHubAppID, extendedOptions.GitHubAppInstallationID, extendedOptions.GitHubAppPemFilePath)
			}
		}

		// This part is the extended implementation that accepts the private key value
		if extendedOptions.GitHubAppPemValue != "" && extendedOptions.GitHubAppClientID != "" {
			appTransport, err = ghinstallation.NewTransport(existingTransport, extendedOptions.GitHubAppClientID, extendedOptions.GitHubAppInstallationID, []byte(extendedOptions.GitHubAppPemValue))
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create transport from GitHub App: %v", err)
		}

		netHttpClient.Transport = appTransport
	}

	// Middleware must be applied after App transport is set, otherwise App token will fail to be
	// renewed with a 400 Bad Request error (even though the request is identical to a successful one.)
	finalTransport := kiotaHttp.NewCustomTransportWithParentTransport(netHttpClient.Transport, middlewares...)
	netHttpClient.Transport = finalTransport

	tokenProviderOptions := []auth.TokenProviderOption{
		auth.WithAPIVersion(extendedOptions.APIVersion),
		auth.WithUserAgent(extendedOptions.UserAgent),
	}

	// If a PAT is provided and GitHub App information is not, configure token authentication
	if extendedOptions.Token != "" && (extendedOptions.GitHubAppInstallationID == 0 && extendedOptions.GitHubAppPemFilePath == "") {
		tokenProviderOptions = append(tokenProviderOptions, auth.WithTokenAuthentication(extendedOptions.Token))
	}

	tokenProvider := auth.NewTokenProvider(tokenProviderOptions...)

	adapter, err := kiotaHttp.NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClient(tokenProvider, nil, nil, netHttpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create request adapter: %v", err)
	}
	if extendedOptions.BaseURL != "" {
		adapter.SetBaseUrl(extendedOptions.BaseURL)
	}

	client := github.NewApiClient(adapter)
	sdkClient := &pkg.Client{
		ApiClient: client,
	}
	return sdkClient, nil
}
