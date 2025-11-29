package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/serverwave/wave-mc-jars-api/internal/models"
)

const (
	purpurAPIBaseURL = "https://api.purpurmc.org/v2/purpur"
)

// PurpurProjectResponse represents the project info from Purpur API
type PurpurProjectResponse struct {
	Project  string   `json:"project"`
	Versions []string `json:"versions"`
}

// PurpurVersionResponse represents version info
type PurpurVersionResponse struct {
	Project string           `json:"project"`
	Version string           `json:"version"`
	Builds  PurpurBuildsInfo `json:"builds"`
}

// PurpurBuildsInfo contains build information
type PurpurBuildsInfo struct {
	Latest string   `json:"latest"`
	All    []string `json:"all"`
}

// PurpurBuildResponse represents a single build
type PurpurBuildResponse struct {
	Project   string         `json:"project"`
	Version   string         `json:"version"`
	Build     string         `json:"build"`
	Result    string         `json:"result"`
	Timestamp int64          `json:"timestamp"`
	Duration  int64          `json:"duration"`
	Commits   []PurpurCommit `json:"commits"`
	Md5       string         `json:"md5"`
}

// PurpurCommit represents a commit
type PurpurCommit struct {
	Author      string `json:"author"`
	Email       string `json:"email"`
	Description string `json:"description"`
	Hash        string `json:"hash"`
	Timestamp   int64  `json:"timestamp"`
}

// PurpurProvider implements Provider for Purpur
type PurpurProvider struct {
	client *http.Client
	config ProviderConfig
}

// NewPurpurProvider creates a new Purpur provider
func NewPurpurProvider(config ProviderConfig) *PurpurProvider {
	return &PurpurProvider{
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		config: config,
	}
}

func (p *PurpurProvider) GetID() string {
	return "purpur"
}

func (p *PurpurProvider) GetName() string {
	return "Purpur"
}

func (p *PurpurProvider) GetCategory() models.Category {
	return models.CategoryPurpur
}

func (p *PurpurProvider) doRequest(ctx context.Context, url string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", p.config.UserAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}

func (p *PurpurProvider) GetVersions(ctx context.Context) ([]models.Version, error) {
	var project PurpurProjectResponse
	if err := p.doRequest(ctx, purpurAPIBaseURL, &project); err != nil {
		return nil, err
	}

	versions := make([]models.Version, 0, len(project.Versions))
	for _, v := range project.Versions {
		versions = append(versions, models.Version{
			ID:     v,
			Type:   models.VersionTypeRelease,
			Stable: true,
		})
	}

	// Purpur API returns oldest first, so reverse to get newest first
	for i, j := 0, len(versions)-1; i < j; i, j = i+1, j-1 {
		versions[i], versions[j] = versions[j], versions[i]
	}

	return versions, nil
}

func (p *PurpurProvider) GetBuilds(ctx context.Context, version string) ([]models.Build, error) {
	url := fmt.Sprintf("%s/%s", purpurAPIBaseURL, version)

	var versionResp PurpurVersionResponse
	if err := p.doRequest(ctx, url, &versionResp); err != nil {
		return nil, err
	}

	builds := make([]models.Build, 0, len(versionResp.Builds.All))
	for _, buildNum := range versionResp.Builds.All {
		var buildInt int
		_, _ = fmt.Sscanf(buildNum, "%d", &buildInt)

		downloadURL := fmt.Sprintf("%s/%s/%s/download", purpurAPIBaseURL, version, buildNum)

		builds = append(builds, models.Build{
			Number:  buildInt,
			Version: version,
			Stable:  true,
			Downloads: []models.Download{
				{
					Name:        fmt.Sprintf("purpur-%s-%s.jar", version, buildNum),
					UpstreamURL: downloadURL,
				},
			},
		})
	}

	// Sort builds by number descending (newest first)
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].Number > builds[j].Number
	})

	return builds, nil
}

func (p *PurpurProvider) GetBuild(ctx context.Context, version string, build int) (*models.Build, error) {
	url := fmt.Sprintf("%s/%s/%d", purpurAPIBaseURL, version, build)

	var buildResp PurpurBuildResponse
	if err := p.doRequest(ctx, url, &buildResp); err != nil {
		return nil, err
	}

	changes := make([]models.Change, 0, len(buildResp.Commits))
	for _, c := range buildResp.Commits {
		changes = append(changes, models.Change{
			Commit:  c.Hash,
			Summary: c.Description,
			Author:  c.Author,
		})
	}

	// Parse timestamp (milliseconds)
	var createdAt time.Time
	if buildResp.Timestamp > 0 {
		createdAt = time.UnixMilli(buildResp.Timestamp)
	}

	downloadURL := fmt.Sprintf("%s/%s/%d/download", purpurAPIBaseURL, version, build)

	return &models.Build{
		Number:    build,
		Version:   version,
		Stable:    buildResp.Result == "SUCCESS",
		CreatedAt: createdAt,
		Downloads: []models.Download{
			{
				Name:        fmt.Sprintf("purpur-%s-%d.jar", version, build),
				UpstreamURL: downloadURL,
			},
		},
		Changes: changes,
	}, nil
}

func (p *PurpurProvider) GetLatestBuild(ctx context.Context, version string) (*models.Build, error) {
	url := fmt.Sprintf("%s/%s", purpurAPIBaseURL, version)

	var versionResp PurpurVersionResponse
	if err := p.doRequest(ctx, url, &versionResp); err != nil {
		return nil, err
	}

	var latestBuild int
	_, _ = fmt.Sscanf(versionResp.Builds.Latest, "%d", &latestBuild)

	return p.GetBuild(ctx, version, latestBuild)
}

func (p *PurpurProvider) GetDownloadURL(_ context.Context, version string, build int) (string, error) {
	return fmt.Sprintf("%s/%s/%d/download", purpurAPIBaseURL, version, build), nil
}
