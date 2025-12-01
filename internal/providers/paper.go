package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ServerwaveHost/wave-mc-jars-api/internal/models"
)

const (
	fillAPIBaseURL = "https://fill.papermc.io/v3"
)

// FillVersionsResponse represents the /v3/projects/{project}/versions response
type FillVersionsResponse struct {
	Versions []struct {
		Builds  []int `json:"builds"`
		Version struct {
			ID   string `json:"id"`
			Java *struct {
				Version struct {
					Minimum int `json:"minimum"`
				} `json:"version"`
			} `json:"java"`
			Support *struct {
				Status string `json:"status"`
				End    string `json:"end"`
			} `json:"support"`
		} `json:"version"`
	} `json:"versions"`
}

// FillBuild represents a build from Fill API v3
type FillBuild struct {
	ID        int                          `json:"id"`
	Channel   string                       `json:"channel"`
	Time      string                       `json:"time"`
	Downloads map[string]FillDownloadEntry `json:"downloads"`
	Changes   []FillChange                 `json:"changes"`
}

// FillDownloadEntry represents a download entry
type FillDownloadEntry struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// FillChange represents a change/commit
type FillChange struct {
	Commit  string `json:"commit"`
	Summary string `json:"summary"`
	Message string `json:"message"`
}

// VersionInfo holds support status and Java version
type VersionInfo struct {
	Supported bool
	Java      int
}

// PaperProvider implements Provider for PaperMC projects using Fill API v3
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

// NewFoliaProvider creates a new Folia provider (uses Fill API)
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

// NewVelocityProvider creates a new Velocity provider (uses Fill API)
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

func (p *PaperProvider) GetFilters() models.CategoryFilters {
	filters := models.CategoryFilters{
		Types:     []models.VersionType{models.VersionTypeRelease, models.VersionTypeSnapshot},
		Channels:  []string{"ALPHA", "BETA", "STABLE", "RECOMMENDED"},
		Stable:    true,
		Supported: true,
		Java:      true,
		Year:      true,
	}

	if p.projectID == "velocity" {
		filters.Types = []models.VersionType{models.VersionTypeSnapshot}
	}

	return filters
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

// fetchAllVersionInfo fetches support status and Java version for all versions in a single API call
func (p *PaperProvider) fetchAllVersionInfo(ctx context.Context) (map[string]VersionInfo, error) {
	url := fmt.Sprintf("%s/projects/%s/versions", fillAPIBaseURL, p.projectID)

	var versionsResp FillVersionsResponse
	if err := p.doRequest(ctx, url, &versionsResp); err != nil {
		return nil, err
	}

	results := make(map[string]VersionInfo)
	for _, v := range versionsResp.Versions {
		info := VersionInfo{}

		// Check support status
		if v.Version.Support != nil && strings.ToUpper(v.Version.Support.Status) == "SUPPORTED" {
			info.Supported = true
		}

		// Get Java version
		if v.Version.Java != nil {
			info.Java = v.Version.Java.Version.Minimum
		}

		results[v.Version.ID] = info
	}

	return results, nil
}

// isStableChannel checks if a channel is considered stable
func isStableChannel(channel string) bool {
	ch := strings.ToUpper(channel)
	return ch == "STABLE" || ch == "RECOMMENDED"
}

// isSnapshotVersion checks if a version string indicates a snapshot/dev version
func isSnapshotVersion(version string) bool {
	v := strings.ToUpper(version)
	return strings.Contains(v, "SNAPSHOT") ||
		strings.Contains(v, "-DEV") ||
		strings.Contains(v, "-BETA") ||
		strings.Contains(v, "-ALPHA") ||
		strings.Contains(v, "-RC") ||
		strings.Contains(v, "-PRE")
}

// getVersionType determines the version type from version string
func getVersionType(version string) models.VersionType {
	v := strings.ToUpper(version)
	if strings.Contains(v, "SNAPSHOT") || strings.Contains(v, "-DEV") || strings.Contains(v, "-PRE") || strings.Contains(v, "-RC") {
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

// parseSemanticVersion parses a version string into comparable parts
func parseSemanticVersion(version string) (major, minor, patch int, preRelease string, preReleaseNum int) {
	v := strings.ToLower(version)

	parts := strings.SplitN(v, "-", 2)
	mainPart := parts[0]
	if len(parts) > 1 {
		prePart := parts[1]
		for i, c := range prePart {
			if c >= '0' && c <= '9' {
				preRelease = prePart[:i]
				preReleaseNum, _ = strconv.Atoi(prePart[i:])
				break
			}
		}
		if preRelease == "" {
			preRelease = prePart
		}
	}

	versionParts := strings.Split(mainPart, ".")
	if len(versionParts) >= 1 {
		major, _ = strconv.Atoi(versionParts[0])
	}
	if len(versionParts) >= 2 {
		minor, _ = strconv.Atoi(versionParts[1])
	}
	if len(versionParts) >= 3 {
		patch, _ = strconv.Atoi(versionParts[2])
	}

	return
}

// compareVersions compares two version strings semantically
func compareVersions(v1, v2 string) int {
	maj1, min1, pat1, pre1, preNum1 := parseSemanticVersion(v1)
	maj2, min2, pat2, pre2, preNum2 := parseSemanticVersion(v2)

	if maj1 != maj2 {
		if maj1 > maj2 {
			return 1
		}
		return -1
	}
	if min1 != min2 {
		if min1 > min2 {
			return 1
		}
		return -1
	}
	if pat1 != pat2 {
		if pat1 > pat2 {
			return 1
		}
		return -1
	}

	if pre1 == "" && pre2 != "" {
		return 1
	}
	if pre1 != "" && pre2 == "" {
		return -1
	}

	preOrder := map[string]int{
		"snapshot": 1,
		"alpha":    2,
		"beta":     3,
		"pre":      4,
		"rc":       5,
	}

	order1 := preOrder[pre1]
	order2 := preOrder[pre2]
	if order1 != order2 {
		if order1 > order2 {
			return 1
		}
		return -1
	}

	if preNum1 != preNum2 {
		if preNum1 > preNum2 {
			return 1
		}
		return -1
	}

	return 0
}

func (p *PaperProvider) GetVersions(ctx context.Context) ([]models.Version, error) {
	// Fetch all version info (support status + Java version) in a single API call
	versionInfoMap, err := p.fetchAllVersionInfo(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]models.Version, 0, len(versionInfoMap))
	for versionID, info := range versionInfoMap {
		versionType := getVersionType(versionID)

		versions = append(versions, models.Version{
			ID:        versionID,
			Type:      versionType,
			Stable:    !isSnapshotVersion(versionID),
			Supported: info.Supported,
			Java:      info.Java,
		})
	}

	// Fetch build info for each version to get release dates and check for stable builds
	type versionBuildInfo struct {
		index       int
		releaseTime time.Time
		hasStable   bool
	}

	results := make(chan versionBuildInfo, len(versions))

	for i := range versions {
		go func(idx int, version models.Version) {
			info := versionBuildInfo{index: idx, hasStable: false}

			buildsURL := fmt.Sprintf("%s/projects/%s/versions/%s/builds", fillAPIBaseURL, p.projectID, version.ID)
			var builds []FillBuild
			if err := p.doRequest(ctx, buildsURL, &builds); err == nil && len(builds) > 0 {
				latestBuild := builds[0]
				if t, err := time.Parse(time.RFC3339, latestBuild.Time); err == nil {
					info.releaseTime = t
				}

				for _, b := range builds {
					if isStableChannel(b.Channel) {
						info.hasStable = true
						break
					}
				}
			}

			results <- info
		}(i, versions[i])
	}

	for range versions {
		info := <-results
		if !info.releaseTime.IsZero() {
			versions[info.index].ReleaseTime = info.releaseTime
		}
		versions[info.index].Stable = info.hasStable
	}

	// Sort by semantic version (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].ID, versions[j].ID) > 0
	})

	return versions, nil
}

func (p *PaperProvider) GetBuilds(ctx context.Context, version string) ([]models.Build, error) {
	url := fmt.Sprintf("%s/projects/%s/versions/%s/builds", fillAPIBaseURL, p.projectID, version)

	var fillBuilds []FillBuild
	if err := p.doRequest(ctx, url, &fillBuilds); err != nil {
		return nil, err
	}

	builds := make([]models.Build, 0, len(fillBuilds))
	for _, b := range fillBuilds {
		buildTime, _ := time.Parse(time.RFC3339, b.Time)

		changes := make([]models.Change, 0, len(b.Changes))
		for _, c := range b.Changes {
			changes = append(changes, models.Change{
				Commit:  c.Commit,
				Summary: c.Summary,
			})
		}

		var downloadURL, downloadName, sha256 string
		if dl, ok := b.Downloads["server:default"]; ok {
			downloadURL = dl.URL
			sha256 = dl.SHA256
			downloadName = fmt.Sprintf("%s-%s-%d.jar", p.projectID, version, b.ID)
		} else if dl, ok := b.Downloads["proxy:default"]; ok {
			downloadURL = dl.URL
			sha256 = dl.SHA256
			downloadName = fmt.Sprintf("%s-%s-%d.jar", p.projectID, version, b.ID)
		}

		downloads := []models.Download{}
		if downloadURL != "" {
			downloads = append(downloads, models.Download{
				Name:        downloadName,
				SHA256:      sha256,
				UpstreamURL: downloadURL,
			})
		}

		builds = append(builds, models.Build{
			Number:    b.ID,
			Version:   version,
			Channel:   b.Channel,
			Stable:    isStableChannel(b.Channel),
			CreatedAt: buildTime,
			Downloads: downloads,
			Changes:   changes,
		})
	}

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

	for i := range builds {
		if builds[i].Stable {
			return &builds[i], nil
		}
	}

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
