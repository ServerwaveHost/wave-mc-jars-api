package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/serverwave/wave-mc-jars-api/internal/models"
)

const (
	mojangVersionManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
)

// MojangVersionManifest represents the Mojang version manifest response
type MojangVersionManifest struct {
	Latest   MojangLatest         `json:"latest"`
	Versions []MojangVersionEntry `json:"versions"`
}

// MojangLatest contains the latest release and snapshot versions
type MojangLatest struct {
	Release  string `json:"release"`
	Snapshot string `json:"snapshot"`
}

// MojangVersionEntry represents a single version in the manifest
type MojangVersionEntry struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	URL             string `json:"url"`
	Time            string `json:"time"`
	ReleaseTime     string `json:"releaseTime"`
	SHA1            string `json:"sha1"`
	ComplianceLevel int    `json:"complianceLevel"`
}

// MojangVersionDetail represents the detailed version information
type MojangVersionDetail struct {
	ID        string                 `json:"id"`
	Downloads MojangVersionDownloads `json:"downloads"`
}

// MojangVersionDownloads contains download URLs for a version
type MojangVersionDownloads struct {
	Server MojangDownloadEntry `json:"server"`
	Client MojangDownloadEntry `json:"client"`
}

// MojangDownloadEntry represents a download entry
type MojangDownloadEntry struct {
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// VanillaProvider implements Provider for Mojang's vanilla server
type VanillaProvider struct {
	client    *http.Client
	config    ProviderConfig
	manifest  *MojangVersionManifest
	cacheTime time.Time
}

// NewVanillaProvider creates a new vanilla provider
func NewVanillaProvider(config ProviderConfig) *VanillaProvider {
	return &VanillaProvider{
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		config: config,
	}
}

func (p *VanillaProvider) GetID() string {
	return "vanilla"
}

func (p *VanillaProvider) GetName() string {
	return "Vanilla"
}

func (p *VanillaProvider) GetCategory() models.Category {
	return models.CategoryVanilla
}

func (p *VanillaProvider) fetchManifest(ctx context.Context) error {
	// Cache for 5 minutes
	if p.manifest != nil && time.Since(p.cacheTime) < 5*time.Minute {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", mojangVersionManifestURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", p.config.UserAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching manifest: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var manifest MojangVersionManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return fmt.Errorf("decoding manifest: %w", err)
	}

	p.manifest = &manifest
	p.cacheTime = time.Now()
	return nil
}

func (p *VanillaProvider) GetVersions(ctx context.Context) ([]models.Version, error) {
	if err := p.fetchManifest(ctx); err != nil {
		return nil, err
	}

	versions := make([]models.Version, 0, len(p.manifest.Versions))
	for _, v := range p.manifest.Versions {
		releaseTime, _ := time.Parse(time.RFC3339, v.ReleaseTime)

		vType := models.VersionTypeRelease
		switch v.Type {
		case "snapshot":
			vType = models.VersionTypeSnapshot
		case "old_beta":
			vType = models.VersionTypeBeta
		case "old_alpha":
			vType = models.VersionTypeAlpha
		}

		versions = append(versions, models.Version{
			ID:          v.ID,
			Type:        vType,
			ReleaseTime: releaseTime,
			Stable:      v.Type == "release",
		})
	}

	return versions, nil
}

func (p *VanillaProvider) fetchVersionDetail(ctx context.Context, versionURL string) (*MojangVersionDetail, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", versionURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", p.config.UserAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching version detail: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var detail MojangVersionDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("decoding version detail: %w", err)
	}

	return &detail, nil
}

func (p *VanillaProvider) findVersionURL(version string) (string, error) {
	if p.manifest == nil {
		return "", fmt.Errorf("manifest not loaded")
	}

	for _, v := range p.manifest.Versions {
		if v.ID == version {
			return v.URL, nil
		}
	}
	return "", fmt.Errorf("version %s not found", version)
}

func (p *VanillaProvider) GetBuilds(ctx context.Context, version string) ([]models.Build, error) {
	if err := p.fetchManifest(ctx); err != nil {
		return nil, err
	}

	versionURL, err := p.findVersionURL(version)
	if err != nil {
		return nil, err
	}

	detail, err := p.fetchVersionDetail(ctx, versionURL)
	if err != nil {
		return nil, err
	}

	// Vanilla has only one "build" per version
	if detail.Downloads.Server.URL == "" {
		return nil, fmt.Errorf("no server download available for version %s", version)
	}

	build := models.Build{
		Number:  1,
		Version: version,
		Stable:  true,
		Downloads: []models.Download{
			{
				Name:        fmt.Sprintf("server-%s.jar", version),
				SHA1:        detail.Downloads.Server.SHA1,
				Size:        detail.Downloads.Server.Size,
				UpstreamURL: detail.Downloads.Server.URL,
			},
		},
	}

	return []models.Build{build}, nil
}

func (p *VanillaProvider) GetBuild(ctx context.Context, version string, build int) (*models.Build, error) {
	builds, err := p.GetBuilds(ctx, version)
	if err != nil {
		return nil, err
	}

	if len(builds) == 0 || build != 1 {
		return nil, fmt.Errorf("build %d not found for version %s", build, version)
	}

	return &builds[0], nil
}

func (p *VanillaProvider) GetLatestBuild(ctx context.Context, version string) (*models.Build, error) {
	return p.GetBuild(ctx, version, 1)
}

func (p *VanillaProvider) GetDownloadURL(ctx context.Context, version string, build int) (string, error) {
	b, err := p.GetBuild(ctx, version, build)
	if err != nil {
		return "", err
	}

	if len(b.Downloads) == 0 {
		return "", fmt.Errorf("no download available")
	}

	return b.Downloads[0].UpstreamURL, nil
}
