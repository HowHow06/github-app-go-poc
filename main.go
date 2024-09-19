package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	kiotaSerialization "github.com/microsoft/kiota-abstractions-go/serialization"
	kiotaHttp "github.com/microsoft/kiota-http-go"
	"github.com/octokit/go-sdk/pkg"

	"bytes"
	"io"
	"net/http"
)

func printRequest(req *http.Request) {
	// Print the request method and URL
	log.Printf("Method: %s\n", req.Method)
	log.Printf("URL: %s\n", req.URL)

	// Print the headers
	log.Println("Headers:")

	for key, values := range req.Header {
		for _, value := range values {
			log.Printf("%s: %s\n", key, value)
		}
	}

	// If the request has a body, read and print it
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			log.Println("Error reading body:", err)
			return
		}
		log.Printf("Body: %s\n", string(body))

		// It's important to reset the body since reading it drains it
		req.Body = io.NopCloser(bytes.NewBuffer(body))
	}
}

type LogHandler struct{}

func (handler LogHandler) Intercept(pipeline kiotaHttp.Pipeline, middlewareIndex int, request *http.Request) (*http.Response, error) {
	printRequest(request)
	log.Println(middlewareIndex)

	resp, err := pipeline.Next(request, middlewareIndex)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

func WithLog() pkg.ClientOptionFunc {
	return func(c *pkg.ClientOptions) {
		c.Middleware = append(c.Middleware, LogHandler{})
	}
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	installationID, err := strconv.ParseInt(os.Getenv("INSTALLATION_ID"), 10, 64)
	if err != nil {
		log.Fatalf("Error parsing installation ID from string to int64: %v", err)
	}

	pathToPem := os.Getenv("PATH_TO_PEM_FILE")
	privateKey, err := os.ReadFile(pathToPem)
	if err != nil {
		log.Fatalf("could not read private key: %s", err)
		return
	}

	pemValue := string(privateKey[:])
	// token expire one hour from the time you create them
	client, err := NewApiClient(
		// pkg.WithUserAgent("my-user-agent"),
		ClientOptionFuncAdapter(pkg.WithRequestTimeout(5*time.Second)),
		// pkg.WithBaseUrl("https://api.github.com"),
		WithGitHubAppAuthenticationUsingPrivateKeyValue(pemValue, os.Getenv("CLIENT_ID"), installationID),

		// pkg.WithGitHubAppAuthentication(os.Getenv("PATH_TO_PEM_FILE"), os.Getenv("CLIENT_ID"), installationID),
	)

	if err != nil {
		log.Fatalf("error creating client: %v", err)
	}

	ownerId := os.Getenv("OWNER_NAME")
	repoId := os.Getenv("REPO_NAME")
	branchName := os.Getenv("BRANCH_NAME")
	// Since Octokit Go SDK is generated using Kiota, refer to https://learn.microsoft.com/en-us/openapi/kiota/quickstarts/go
	// GET /repos/{owner}/{repo}/branches/{branch}/protection
	response, err := client.Repos().ByOwnerId(ownerId).ByRepoId(repoId).Branches().ByBranch(branchName).Protection().Get(context.Background(), nil)

	if err != nil {
		log.Fatalf("error getting branch protection rules: %v", err)
	}
	result, _ := kiotaSerialization.SerializeToJson(response)

	log.Println(string(result))
}
