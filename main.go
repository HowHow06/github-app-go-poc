package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	kiotaSerialization "github.com/microsoft/kiota-abstractions-go/serialization"
	kiotaHttp "github.com/microsoft/kiota-http-go"
	"github.com/octokit/go-sdk/pkg"
	"github.com/octokit/go-sdk/pkg/github/models"

	"bytes"
	"io"
	"net/http"
	"reflect"
)

const (
	BranchNotProtectedMessage = "Branch not protected"
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

func printErrorProperties(err any) {
	// Use reflection to inspect the error's properties
	val := reflect.ValueOf(err)
	typ := reflect.TypeOf(err)

	if typ.Kind() == reflect.Ptr {
		// Dereference the pointer to get the underlying type
		val = val.Elem()
		typ = typ.Elem()
	}

	// Print the type and its fields
	log.Printf("Type: %s\n", typ.Name())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)

		// Check if the field is exported (public)
		if field.PkgPath == "" {
			// Exported fields have an empty PkgPath
			log.Printf("%s: %v\n", field.Name, value.Interface())
		} else {
			// Unexported fields are skipped
			log.Printf("%s: (unexported)\n", field.Name)
		}
	}
}

// func printStructFields(s any, indent string) {
// 	val := reflect.ValueOf(s)
// 	typ := reflect.TypeOf(s)

// 	// If it's a pointer, dereference it
// 	if typ.Kind() == reflect.Ptr {
// 		val = val.Elem()
// 		typ = typ.Elem()
// 	}

// 	// Iterate through the struct's fields
// 	for i := 0; i < typ.NumField(); i++ {
// 		field := typ.Field(i)
// 		value := val.Field(i)

// 		// Check if the field is exported (public)
// 		if field.PkgPath == "" {
// 			// If the field is a struct, recursively print its fields
// 			if value.Kind() == reflect.Struct {
// 				log.Printf("%s%s (struct):\n", indent, field.Name)
// 				printStructFields(value.Interface(), indent+"  ") // Recursive call with increased indent
// 			} else {
// 				log.Printf("%s%s: %v\n", indent, field.Name, value.Interface())
// 			}
// 		} else {
// 			// Unexported field, cannot be accessed
// 			log.Printf("%s%s: (unexported)\n", indent, field.Name)
// 		}
// 	}
// }

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

func GetGhbpRules(client *pkg.Client, owner, repo, branch string) (response []byte, err error) {
	// Since Octokit Go SDK is generated using Kiota, refer to https://learn.microsoft.com/en-us/openapi/kiota/quickstarts/go
	// GET /repos/{owner}/{repo}/branches/{branch}/protection
	responseModel, err := client.Repos().ByOwnerId(owner).ByRepoId(repo).Branches().ByBranch(branch).Protection().Get(context.Background(), nil)
	if err != nil {
		if apiError, ok := err.(*models.BasicError); ok {
			statusCode := apiError.ResponseStatusCode
			errorMessage := apiError.GetMessage()
			// Branch has no protection rules
			if statusCode == http.StatusNotFound && *errorMessage == BranchNotProtectedMessage {
				response = []byte("{}")
				err = nil
				return
			}
		}
		return
	}

	response, err = kiotaSerialization.SerializeToJson(responseModel)
	return
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
		// Check if the error contains an HTTP response
		if basicError, ok := err.(*models.BasicError); ok {
			// Access the response and status code
			// statusCode := basicError.ResponseStatusCode
			log.Println(basicError.GetAdditionalData())
			log.Printf("Message: %s\n", *basicError.GetMessage())                    // Not Found, OR Branch not protected
			log.Printf("Status: %s\n", *basicError.GetStatus())                      // 404, OR basicError.ResponseStatusCode
			log.Printf("Documentation URL: %s\n", *basicError.GetDocumentationUrl()) // https://docs.github.com/rest/branches/branch-protection#get-branch-protection

		} else {
			// Handle non-API error
			printErrorProperties(err)
			log.Fatalf("Non-API error: %v", err)
		}
	}

	result, _ := kiotaSerialization.SerializeToJson(response)

	log.Println(string(result))

	responseBytes, err := GetGhbpRules(client, ownerId, repoId, branchName)
	if err != nil {
		// Check if the error contains an HTTP response
		if _, ok := err.(*models.BasicError); ok {
		} else {
			// Handle non-API error
			printErrorProperties(err)
			log.Fatalf("Non-API error: %v", err)
		}
	}
	fmt.Printf("responseBytes: %v\n", string(responseBytes))
}
