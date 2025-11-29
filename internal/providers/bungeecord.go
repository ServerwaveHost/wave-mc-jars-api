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
	bungeecordJenkinsURL = "https://ci.md-5.net/job/BungeeCord"
)

// JenkinsJobInfo represents Jenkins job information
type JenkinsJobInfo struct {
	Builds []JenkinsBuildRef `json:"builds"`
}

// JenkinsBuildRef represents a reference to a build
type JenkinsBuildRef struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// JenkinsBuildInfo represents detailed build information
type JenkinsBuildInfo struct {
	Number    int               `json:"number"`
	Result    string            `json:"result"`
	Timestamp int64             `json:"timestamp"`
	Artifacts []JenkinsArtifact `json:"artifacts"`
}

// JenkinsArtifact represents a build artifact
type JenkinsArtifact struct {
	DisplayPath  string `json:"displayPath"`
	FileName     string `json:"fileName"`
	RelativePath string `json:"relativePath"`
}

// BungeeCordProvider implements Provider for BungeeCord
type BungeeCordProvider struct {
	client *http.Client
	config ProviderConfig
}

// NewBungeeCordProvider creates a new BungeeCord provider
func NewBungeeCordProvider(config ProviderConfig) *BungeeCordProvider {
	return &BungeeCordProvider{
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		config: config,
	}
}

func (p *BungeeCordProvider) GetID() string {
	return "bungeecord"
}

func (p *BungeeCordProvider) GetName() string {
	return "BungeeCord"
}

func (p *BungeeCordProvider) GetCategory() models.Category {
	return models.CategoryBungeeCord
}

func (p *BungeeCordProvider) doRequest(ctx context.Context, url string, target interface{}) error {
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

// BungeeCord doesn't have traditional "versions" like MC - it's continuously updated
// We provide a "latest" version that always gets the newest build
func (p *BungeeCordProvider) GetVersions(_ context.Context) ([]models.Version, error) {
	versions := []models.Version{
		{
			ID:          "latest",
			Type:        models.VersionTypeRelease,
			Stable:      true,
			ReleaseTime: time.Now(),
		},
	}

	return versions, nil
}

func (p *BungeeCordProvider) GetBuilds(ctx context.Context, version string) ([]models.Build, error) {
	url := fmt.Sprintf("%s/api/json?tree=builds[number,url]", bungeecordJenkinsURL)

	var jobInfo JenkinsJobInfo
	if err := p.doRequest(ctx, url, &jobInfo); err != nil {
		return nil, err
	}

	// Limit to last 50 builds
	maxBuilds := 50
	if len(jobInfo.Builds) < maxBuilds {
		maxBuilds = len(jobInfo.Builds)
	}

	builds := make([]models.Build, 0, maxBuilds)
	for i := 0; i < maxBuilds; i++ {
		buildRef := jobInfo.Builds[i]

		// Get build details
		buildURL := fmt.Sprintf("%s/%d/api/json", bungeecordJenkinsURL, buildRef.Number)
		var buildInfo JenkinsBuildInfo
		if err := p.doRequest(ctx, buildURL, &buildInfo); err != nil {
			continue
		}

		// Find BungeeCord.jar artifact
		var jarArtifact *JenkinsArtifact
		for _, a := range buildInfo.Artifacts {
			if a.FileName == "BungeeCord.jar" {
				artifact := a
				jarArtifact = &artifact
				break
			}
		}

		if jarArtifact == nil {
			continue
		}

		downloadURL := fmt.Sprintf("%s/%d/artifact/%s", bungeecordJenkinsURL, buildRef.Number, jarArtifact.RelativePath)

		builds = append(builds, models.Build{
			Number:    buildRef.Number,
			Version:   version,
			Stable:    buildInfo.Result == "SUCCESS",
			CreatedAt: time.UnixMilli(buildInfo.Timestamp),
			Downloads: []models.Download{
				{
					Name:        fmt.Sprintf("BungeeCord-%d.jar", buildRef.Number),
					UpstreamURL: downloadURL,
				},
			},
		})
	}

	// Sort by build number descending (newest first)
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].Number > builds[j].Number
	})

	return builds, nil
}

func (p *BungeeCordProvider) GetBuild(ctx context.Context, version string, build int) (*models.Build, error) {
	buildURL := fmt.Sprintf("%s/%d/api/json", bungeecordJenkinsURL, build)

	var buildInfo JenkinsBuildInfo
	if err := p.doRequest(ctx, buildURL, &buildInfo); err != nil {
		return nil, err
	}

	// Find BungeeCord.jar artifact
	var jarArtifact *JenkinsArtifact
	for _, a := range buildInfo.Artifacts {
		if a.FileName == "BungeeCord.jar" {
			artifact := a
			jarArtifact = &artifact
			break
		}
	}

	if jarArtifact == nil {
		return nil, fmt.Errorf("no BungeeCord.jar artifact found for build %d", build)
	}

	downloadURL := fmt.Sprintf("%s/%d/artifact/%s", bungeecordJenkinsURL, build, jarArtifact.RelativePath)

	return &models.Build{
		Number:    build,
		Version:   version,
		Stable:    buildInfo.Result == "SUCCESS",
		CreatedAt: time.UnixMilli(buildInfo.Timestamp),
		Downloads: []models.Download{
			{
				Name:        fmt.Sprintf("BungeeCord-%d.jar", build),
				UpstreamURL: downloadURL,
			},
		},
	}, nil
}

func (p *BungeeCordProvider) GetLatestBuild(ctx context.Context, version string) (*models.Build, error) {
	url := fmt.Sprintf("%s/api/json?tree=lastSuccessfulBuild[number]", bungeecordJenkinsURL)

	var result struct {
		LastSuccessfulBuild struct {
			Number int `json:"number"`
		} `json:"lastSuccessfulBuild"`
	}

	if err := p.doRequest(ctx, url, &result); err != nil {
		return nil, err
	}

	return p.GetBuild(ctx, version, result.LastSuccessfulBuild.Number)
}

func (p *BungeeCordProvider) GetDownloadURL(ctx context.Context, version string, build int) (string, error) {
	b, err := p.GetBuild(ctx, version, build)
	if err != nil {
		return "", err
	}

	if len(b.Downloads) == 0 {
		return "", fmt.Errorf("no download available")
	}

	return b.Downloads[0].UpstreamURL, nil
}
