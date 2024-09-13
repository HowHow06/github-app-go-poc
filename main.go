package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	kiotaSerialization "github.com/microsoft/kiota-abstractions-go/serialization"
	"github.com/octokit/go-sdk/pkg"
)

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

	client, err := pkg.NewApiClient(
		// pkg.WithUserAgent("my-user-agent"),
		pkg.WithRequestTimeout(5*time.Second),
		// pkg.WithBaseUrl("https://api.github.com"),
		pkg.WithGitHubAppAuthentication(os.Getenv("PATH_TO_PEM_FILE"), os.Getenv("CLIENT_ID"), installationID),
	)

	if err != nil {
		log.Fatalf("error creating client: %v", err)
	}

	ownerId := os.Getenv("OWNER_NAME")
	repoId := os.Getenv("REPO_NAME")
	branchName := os.Getenv("BRANCH_NAME")
	response, err := client.Repos().ByOwnerId(ownerId).ByRepoId(repoId).Branches().ByBranch(branchName).Protection().Get(context.Background(), nil)

	if err != nil {
		log.Fatalf("error getting branch protection rules: %v", err)
	}
	result, _ := kiotaSerialization.SerializeToJson(response)

	log.Println(string(result))
}
