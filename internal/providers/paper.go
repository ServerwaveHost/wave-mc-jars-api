package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ServerwaveHost/wave-mc-jars-api/internal/models"
)

const (
	paperAPIBaseURL = "https://api.papermc.io/v2"
)

// PaperProjectResponse represents a project from Paper API
type PaperProjectResponse struct {
	ProjectID     string   `json:"project_id"`
	ProjectName   string   `json:"project_name"`
	VersionGroups []string `json:"version_groups"`
	Versions      []string `json:"versions"`
}

// PaperBuildsResponse represents builds for a version
type PaperBuildsResponse struct {
	ProjectID   string       `json:"project_id"`
	ProjectName string       `json:"project_name"`
	Version     string       `json:"version"`
	Builds      []PaperBuild `json:"builds"`
}

// PaperBuild represents a single build
type PaperBuild struct {
	Build     int            `json:"build"`
	Time      string         `json:"time"`
	Channel   string         `json:"channel"`
	Promoted  bool           `json:"promoted"`
	Changes   []PaperChange  `json:"changes"`
	Downloads PaperDownloads `json:"downloads"`
}

// PaperChange represents a change in a build
type PaperChange struct {
	Commit  string `json:"commit"`
	Summary string `json:"summary"`
	Message string `json:"message"`
}

// PaperDownloads contains download info
type PaperDownloads struct {
	Application PaperDownloadEntry `json:"application"`
}

// PaperDownloadEntry represents a download entry
type PaperDownloadEntry struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

// PaperProvider implements Provider for PaperMC
type PaperProvider struct {
	client    *http.Client
	config    ProviderConfig
	projectID string
	category  models.Category
}

// NewPaperProvider creates a new Paper provider
func NewPaperProvider(config ProviderConfig) *PaperProvider {
	return &PaperProvider{
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		config:    config,
		projectID: "paper",
		category:  models.CategoryPaper,
	}
}

// NewFoliaProvider creates a new Folia provider (uses Paper API)
func NewFoliaProvider(config ProviderConfig) *PaperProvider {
	return &PaperProvider{
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		config:    config,
		projectID: "folia",
		category:  models.CategoryFolia,
	}
}

// NewVelocityProvider creates a new Velocity provider (uses Paper API)
func NewVelocityProvider(config ProviderConfig) *PaperProvider {
	return &PaperProvider{
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		config:    config,
		projectID: "velocity",
		category:  models.CategoryVelocity,
	}
}

func (p *PaperProvider) GetID() string {
	return p.projectID
}

func (p *PaperProvider) GetName() string {
	switch p.projectID {
	case "paper":
		return "Paper"
	case "folia":
		return "Folia"
	case "velocity":
		return "Velocity"
	default:
		return p.projectID
	}
}

func (p *PaperProvider) GetCategory() models.Category {
	return p.category
}

func (p *PaperProvider) doRequest(ctx context.Context, url string, target interface{}) error {
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

// isSnapshotVersion checks if a version string indicates a snapshot/dev version
func isSnapshotVersion(version string) bool {
	v := strings.ToUpper(version)
	return strings.Contains(v, "SNAPSHOT") ||
		strings.Contains(v, "-DEV") ||
		strings.Contains(v, "-BETA") ||
		strings.Contains(v, "-ALPHA") ||
		strings.Contains(v, "-RC")
}

// getVersionType determines the version type from version string
func getVersionType(version string) models.VersionType {
	v := strings.ToUpper(version)
	if strings.Contains(v, "SNAPSHOT") || strings.Contains(v, "-DEV") {
		return models.VersionTypeSnapshot
	}
	if strings.Contains(v, "-BETA") {
		return models.VersionTypeBeta
	}
	if strings.Contains(v, "-ALPHA") {
		return models.VersionTypeAlpha
	}
	return models.VersionTypeRelease
}

func (p *PaperProvider) GetVersions(ctx context.Context) ([]models.Version, error) {
	url := fmt.Sprintf("%s/projects/%s", paperAPIBaseURL, p.projectID)

	var project PaperProjectResponse
	if err := p.doRequest(ctx, url, &project); err != nil {
		return nil, err
	}

	versions := make([]models.Version, 0, len(project.Versions))
	for _, v := range project.Versions {
		// Determine version type and stability from version string
		versionType := getVersionType(v)
		isStable := !isSnapshotVersion(v)

		versions = append(versions, models.Version{
			ID:     v,
			Type:   versionType,
			Stable: isStable, // Will be further refined when we fetch builds info
		})
	}

	// Paper API returns oldest first, so reverse to get newest first
	for i, j := 0, len(versions)-1; i < j; i, j = i+1, j-1 {
		versions[i], versions[j] = versions[j], versions[i]
	}

	// Fetch build info for each version to get release dates and stability
	// We do this in parallel for performance
	type versionInfo struct {
		index       int
		releaseTime time.Time
		hasStable   bool
	}

	results := make(chan versionInfo, len(versions))

	for i := range versions {
		go func(idx int, version models.Version) {
			info := versionInfo{index: idx, hasStable: version.Stable}

			// Fetch builds to get the latest build time and check for stable builds
			buildsURL := fmt.Sprintf("%s/projects/%s/versions/%s/builds", paperAPIBaseURL, p.projectID, version.ID)
			var buildsResp PaperBuildsResponse
			if err := p.doRequest(ctx, buildsURL, &buildsResp); err == nil && len(buildsResp.Builds) > 0 {
				// Get the latest build time as the version release time
				latestBuild := buildsResp.Builds[len(buildsResp.Builds)-1]
				if t, err := time.Parse(time.RFC3339, latestBuild.Time); err == nil {
					info.releaseTime = t
				}

				// For non-snapshot versions, check if any build is stable (channel == "default")
				// For snapshot versions, they're inherently not stable
				if !isSnapshotVersion(version.ID) {
					info.hasStable = false
					for _, b := range buildsResp.Builds {
						if b.Channel == "default" {
							info.hasStable = true
							break
						}
					}
				} else {
					info.hasStable = false
				}
			}

			results <- info
		}(i, versions[i])
	}

	// Collect results
	for range versions {
		info := <-results
		if !info.releaseTime.IsZero() {
			versions[info.index].ReleaseTime = info.releaseTime
		}
		versions[info.index].Stable = info.hasStable
	}

	// Re-sort by release time (newest first) if we have dates
	sort.Slice(versions, func(i, j int) bool {
		// If both have release times, sort by time
		if !versions[i].ReleaseTime.IsZero() && !versions[j].ReleaseTime.IsZero() {
			return versions[i].ReleaseTime.After(versions[j].ReleaseTime)
		}
		// Otherwise keep original order (already reversed)
		return i < j
	})

	return versions, nil
}

func (p *PaperProvider) GetBuilds(ctx context.Context, version string) ([]models.Build, error) {
	url := fmt.Sprintf("%s/projects/%s/versions/%s/builds", paperAPIBaseURL, p.projectID, version)

	var buildsResp PaperBuildsResponse
	if err := p.doRequest(ctx, url, &buildsResp); err != nil {
		return nil, err
	}

	builds := make([]models.Build, 0, len(buildsResp.Builds))
	for _, b := range buildsResp.Builds {
		buildTime, _ := time.Parse(time.RFC3339, b.Time)

		changes := make([]models.Change, 0, len(b.Changes))
		for _, c := range b.Changes {
			changes = append(changes, models.Change{
				Commit:  c.Commit,
				Summary: c.Summary,
			})
		}

		downloadURL := fmt.Sprintf("%s/projects/%s/versions/%s/builds/%d/downloads/%s",
			paperAPIBaseURL, p.projectID, version, b.Build, b.Downloads.Application.Name)

		downloads := []models.Download{
			{
				Name:        b.Downloads.Application.Name,
				SHA256:      b.Downloads.Application.SHA256,
				UpstreamURL: downloadURL,
			},
		}

		builds = append(builds, models.Build{
			Number:    b.Build,
			Version:   version,
			Channel:   b.Channel,
			Stable:    b.Channel == "default",
			CreatedAt: buildTime,
			Downloads: downloads,
			Changes:   changes,
		})
	}

	// Sort builds by number descending (newest first)
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].Number > builds[j].Number
	})

	return builds, nil
}

func (p *PaperProvider) GetBuild(ctx context.Context, version string, build int) (*models.Build, error) {
	builds, err := p.GetBuilds(ctx, version)
	if err != nil {
		return nil, err
	}

	for i := range builds {
		if builds[i].Number == build {
			return &builds[i], nil
		}
	}

	return nil, fmt.Errorf("build %d not found for version %s", build, version)
}

func (p *PaperProvider) GetLatestBuild(ctx context.Context, version string) (*models.Build, error) {
	builds, err := p.GetBuilds(ctx, version)
	if err != nil {
		return nil, err
	}

	if len(builds) == 0 {
		return nil, fmt.Errorf("no builds found for version %s", version)
	}

	// First build is now the latest (sorted descending)
	return &builds[0], nil
}

func (p *PaperProvider) GetDownloadURL(ctx context.Context, version string, build int) (string, error) {
	b, err := p.GetBuild(ctx, version, build)
	if err != nil {
		return "", err
	}

	if len(b.Downloads) == 0 {
		return "", fmt.Errorf("no download available")
	}

	return b.Downloads[0].UpstreamURL, nil
}
