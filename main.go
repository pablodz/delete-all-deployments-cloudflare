package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	cloudflareAPIBaseURL   = "https://api.cloudflare.com/client/v4"
	unknownAPIErrorMessage = "unknown Cloudflare API error"
	maxListAttempts        = 5
	deploymentsPerPage     = 10
	paginationBatchSize    = 3
	batchMaxResults        = paginationBatchSize * deploymentsPerPage
)

var (
	requestDelayBetweenCalls = 500 * time.Millisecond
	listRetryInitialDelay    = 1 * time.Second
)

type config struct {
	apiToken                 string
	accountID                string
	pagesProjectName         string
	deleteAliasedDeployments bool
}

type cloudflareAPIError struct {
	Message string `json:"message"`
}

type deploymentIdentifier struct {
	ID string `json:"id"`
}

type projectResult struct {
	CanonicalDeployment *deploymentIdentifier `json:"canonical_deployment"`
}

type projectDetailsResponse struct {
	Success bool                 `json:"success"`
	Errors  []cloudflareAPIError `json:"errors"`
	Result  projectResult        `json:"result"`
}

type listDeploymentsResponse struct {
	Success bool                   `json:"success"`
	Errors  []cloudflareAPIError   `json:"errors"`
	Result  []deploymentIdentifier `json:"result"`
}

type deleteDeploymentResponse struct {
	Success bool                 `json:"success"`
	Errors  []cloudflareAPIError `json:"errors"`
}

func main() {
	appConfig := loadConfigFromEnvironment()
	httpClient := &http.Client{Timeout: 30 * time.Second}

	for {
		productionDeploymentID, err := fetchProductionDeploymentID(httpClient, appConfig)
		if err != nil {
			log.Printf("Warning: unable to fetch production deployment: %v", err)
		}

		if productionDeploymentID != "" {
			log.Printf("Live production deployment will be preserved: %s", productionDeploymentID)
		}

		deploymentIDs, err := listDeploymentIDs(httpClient, appConfig, batchMaxResults)
		if err != nil {
			log.Fatalf("failed to list deployments: %v", err)
		}

		if len(deploymentIDs) == 0 {
			log.Println("No deployments found. Nothing to delete.")
			return
		}

		deletedCount := deleteDeployments(httpClient, appConfig, deploymentIDs, productionDeploymentID)

		if deletedCount == 0 {
			log.Println("No deployments were deleted in this batch. Stopping.")
			return
		}
	}
}

func loadConfigFromEnvironment() config {
	appConfig := config{
		apiToken:                 os.Getenv("CF_API_TOKEN"),
		accountID:                os.Getenv("CF_ACCOUNT_ID"),
		pagesProjectName:         os.Getenv("CF_PAGES_PROJECT_NAME"),
		deleteAliasedDeployments: strings.EqualFold(os.Getenv("CF_DELETE_ALIASED_DEPLOYMENTS"), "true"),
	}

	if appConfig.apiToken == "" {
		log.Fatal("please set CF_API_TOKEN as an environment variable")
	}
	if appConfig.accountID == "" {
		log.Fatal("please set CF_ACCOUNT_ID as an environment variable")
	}
	if appConfig.pagesProjectName == "" {
		log.Fatal("please set CF_PAGES_PROJECT_NAME as an environment variable")
	}

	return appConfig
}

func projectBaseURL(appConfig config) string {
	return fmt.Sprintf(
		"%s/accounts/%s/pages/projects/%s",
		cloudflareAPIBaseURL,
		appConfig.accountID,
		appConfig.pagesProjectName,
	)
}

func deploymentsEndpoint(appConfig config) string {
	return projectBaseURL(appConfig) + "/deployments"
}

func newAuthenticatedRequest(method string, url string, appConfig config) (*http.Request, error) {
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+appConfig.apiToken)
	return request, nil
}

func fetchProductionDeploymentID(httpClient *http.Client, appConfig config) (string, error) {
	request, err := newAuthenticatedRequest(http.MethodGet, projectBaseURL(appConfig), appConfig)
	if err != nil {
		return "", err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var payload projectDetailsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("failed to decode project details response (status %d): %w", response.StatusCode, err)
	}
	if !payload.Success {
		return "", errors.New(firstAPIErrorMessage(payload.Errors))
	}

	if payload.Result.CanonicalDeployment == nil {
		return "", nil
	}

	return payload.Result.CanonicalDeployment.ID, nil
}

func listDeploymentIDs(httpClient *http.Client, appConfig config, maxResults int) ([]string, error) {
	log.Printf("Listing up to %d deployments for project %s", maxResults, appConfig.pagesProjectName)

	totalPages := (maxResults + deploymentsPerPage - 1) / deploymentsPerPage
	deploymentIDs := make([]string, 0, maxResults)

	for pageNumber := 1; pageNumber <= totalPages; pageNumber++ {
		deploymentsOnPage, err := listDeploymentPageWithRetry(httpClient, appConfig, pageNumber)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", pageNumber, err)
		}

		if len(deploymentsOnPage) == 0 {
			break
		}

		for _, deployment := range deploymentsOnPage {
			deploymentIDs = append(deploymentIDs, deployment.ID)
			if len(deploymentIDs) >= maxResults {
				break
			}
		}

		log.Printf("Fetched %d deployment(s) so far", len(deploymentIDs))

		if len(deploymentIDs) >= maxResults {
			break
		}

		if pageNumber < totalPages {
			time.Sleep(requestDelayBetweenCalls)
		}
	}

	return deploymentIDs, nil
}

func listDeploymentPageWithRetry(httpClient *http.Client, appConfig config, pageNumber int) ([]deploymentIdentifier, error) {
	retryDelay := listRetryInitialDelay
	var lastError error

	for attempt := 1; attempt <= maxListAttempts; attempt++ {
		deploymentsOnPage, err := listDeploymentPage(httpClient, appConfig, pageNumber)
		if err == nil {
			return deploymentsOnPage, nil
		}

		lastError = err
		if attempt < maxListAttempts {
			log.Printf("List request for page %d failed (attempt %d/%d): %v", pageNumber, attempt, maxListAttempts, err)
			time.Sleep(retryDelay)
			retryDelay *= 2
		}
	}

	return nil, fmt.Errorf("retries exhausted: %w", lastError)
}

func listDeploymentPage(httpClient *http.Client, appConfig config, pageNumber int) ([]deploymentIdentifier, error) {
	pageURL := fmt.Sprintf("%s?per_page=%d&page=%d", deploymentsEndpoint(appConfig), deploymentsPerPage, pageNumber)
	request, err := newAuthenticatedRequest(http.MethodGet, pageURL, appConfig)
	if err != nil {
		return nil, err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var payload listDeploymentsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode list response (status %d): %w", response.StatusCode, err)
	}
	if !payload.Success {
		return nil, errors.New(firstAPIErrorMessage(payload.Errors))
	}

	return payload.Result, nil
}

func deleteDeployments(
	httpClient *http.Client,
	appConfig config,
	deploymentIDs []string,
	productionDeploymentID string,
) int {
	deletedCount := 0

	for _, deploymentID := range deploymentIDs {
		if productionDeploymentID != "" && deploymentID == productionDeploymentID {
			log.Printf("Skipping live production deployment: %s", deploymentID)
			continue
		}

		if err := deleteSingleDeployment(httpClient, appConfig, deploymentID); err != nil {
			log.Printf("Failed to delete deployment %s: %v", deploymentID, err)
			continue
		}

		log.Printf("Deleted deployment %s from project %s", deploymentID, appConfig.pagesProjectName)
		deletedCount++
		time.Sleep(requestDelayBetweenCalls)
	}

	return deletedCount
}

func deleteSingleDeployment(httpClient *http.Client, appConfig config, deploymentID string) error {
	deleteURL := fmt.Sprintf("%s/%s", deploymentsEndpoint(appConfig), deploymentID)
	if appConfig.deleteAliasedDeployments {
		deleteURL += "?force=true"
	}

	request, err := newAuthenticatedRequest(http.MethodDelete, deleteURL, appConfig)
	if err != nil {
		return err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	var payload deleteDeploymentResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return fmt.Errorf("failed to decode delete response (status %d): %w", response.StatusCode, err)
	}
	if !payload.Success {
		return errors.New(firstAPIErrorMessage(payload.Errors))
	}

	return nil
}

func firstAPIErrorMessage(apiErrors []cloudflareAPIError) string {
	if len(apiErrors) > 0 && apiErrors[0].Message != "" {
		return apiErrors[0].Message
	}

	return unknownAPIErrorMessage
}
