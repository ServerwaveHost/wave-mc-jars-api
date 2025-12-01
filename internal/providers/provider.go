package providers

import (
	"context"

	"github.com/ServerwaveHost/wave-mc-jars-api/internal/models"
)

// Provider defines the interface for fetching Minecraft server JARs from different sources
type Provider interface {
	// GetID returns the unique identifier for this provider
	GetID() string

	// GetName returns the human-readable name of this provider
	GetName() string

	// GetCategory returns the category this provider belongs to
	GetCategory() models.Category

	// GetFilters returns the available filters for this provider
	GetFilters() models.CategoryFilters

	// GetVersions returns all available Minecraft versions for this provider
	GetVersions(ctx context.Context) ([]models.Version, error)

	// GetBuilds returns all builds for a specific Minecraft version
	GetBuilds(ctx context.Context, version string) ([]models.Build, error)

	// GetBuild returns a specific build for a version
	GetBuild(ctx context.Context, version string, build int) (*models.Build, error)

	// GetLatestBuild returns the latest build for a version
	GetLatestBuild(ctx context.Context, version string) (*models.Build, error)

	// GetDownloadURL returns the download URL for a specific build
	GetDownloadURL(ctx context.Context, version string, build int) (string, error)
}

// ProviderConfig contains configuration for providers
type ProviderConfig struct {
	UserAgent string
	Timeout   int
}

// DefaultConfig returns the default provider configuration
func DefaultConfig() ProviderConfig {
	return ProviderConfig{
		UserAgent: "JarVault/1.0.0 (https://github.com/ServerwaveHost/wave-mc-jars-api; contact@serverwave.com)",
		Timeout:   30,
	}
}
