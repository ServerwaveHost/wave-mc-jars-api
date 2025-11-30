package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ServerwaveHost/wave-mc-jars-api/internal/cache"
	"github.com/ServerwaveHost/wave-mc-jars-api/internal/java"
	"github.com/ServerwaveHost/wave-mc-jars-api/internal/models"
	"github.com/ServerwaveHost/wave-mc-jars-api/internal/providers"
)

// JarsService provides high-level operations for Minecraft JAR management
type JarsService struct {
	registry *providers.Registry
	cache    cache.Cache
}

// NewJarsService creates a new service instance
func NewJarsService(registry *providers.Registry, c cache.Cache) *JarsService {
	return &JarsService{
		registry: registry,
		cache:    c,
	}
}

// GetCategories returns all available categories
func (s *JarsService) GetCategories(_ context.Context) []models.CategoryInfo {
	providersList := s.registry.List()
	categories := make([]models.CategoryInfo, 0, len(providersList))

	for _, p := range providersList {
		categories = append(categories, models.CategoryInfo{
			ID:          p.GetCategory(),
			Name:        p.GetName(),
			Description: getCategoryDescription(p.GetCategory()),
		})
	}

	// Sort by name
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	return categories
}

// GetCategory returns a specific category
func (s *JarsService) GetCategory(_ context.Context, categoryID string) (*models.CategoryInfo, error) {
	p, err := s.registry.Get(categoryID)
	if err != nil {
		return nil, err
	}

	return &models.CategoryInfo{
		ID:          p.GetCategory(),
		Name:        p.GetName(),
		Description: getCategoryDescription(p.GetCategory()),
	}, nil
}

// GetVersions returns all versions for a category
func (s *JarsService) GetVersions(ctx context.Context, categoryID string) ([]models.Version, error) {
	cacheKey := fmt.Sprintf("versions:%s", categoryID)

	var versions []models.Version
	if err := s.cache.Get(ctx, cacheKey, &versions); err == nil {
		return versions, nil
	}

	p, err := s.registry.Get(categoryID)
	if err != nil {
		return nil, err
	}

	versions, err = p.GetVersions(ctx)
	if err != nil {
		return nil, err
	}

	// Add Java requirements to each version
	for i := range versions {
		versions[i].Java = java.GetRequirement(versions[i].ID, p.GetCategory())
	}

	_ = s.cache.Set(ctx, cacheKey, versions)
	return versions, nil
}

// GetVersionsFiltered returns versions filtered by options
func (s *JarsService) GetVersionsFiltered(ctx context.Context, categoryID string, opts VersionFilterOptions) ([]models.Version, error) {
	versions, err := s.GetVersions(ctx, categoryID)
	if err != nil {
		return nil, err
	}

	filtered := make([]models.Version, 0)
	for _, v := range versions {
		// Filter by type
		if opts.Type != nil && v.Type != *opts.Type {
			continue
		}

		// Filter by stability
		if opts.StableOnly && !v.Stable {
			continue
		}

		// Filter by Java version
		if opts.Java != nil && v.Java != *opts.Java {
			continue
		}

		// Filter by date range
		if opts.After != nil && !v.ReleaseTime.IsZero() && v.ReleaseTime.Before(*opts.After) {
			continue
		}
		if opts.Before != nil && !v.ReleaseTime.IsZero() && v.ReleaseTime.After(*opts.Before) {
			continue
		}

		// Filter by year
		if opts.MinYear != nil && !v.ReleaseTime.IsZero() && v.ReleaseTime.Year() < *opts.MinYear {
			continue
		}
		if opts.MaxYear != nil && !v.ReleaseTime.IsZero() && v.ReleaseTime.Year() > *opts.MaxYear {
			continue
		}

		filtered = append(filtered, v)
	}

	return filtered, nil
}

// GetBuilds returns all builds for a category version
func (s *JarsService) GetBuilds(ctx context.Context, categoryID, version string) ([]models.Build, error) {
	cacheKey := fmt.Sprintf("builds:%s:%s", categoryID, version)

	var builds []models.Build
	if err := s.cache.Get(ctx, cacheKey, &builds); err == nil {
		return builds, nil
	}

	p, err := s.registry.Get(categoryID)
	if err != nil {
		return nil, err
	}

	builds, err = p.GetBuilds(ctx, version)
	if err != nil {
		return nil, err
	}

	// Add Java requirements to each build
	javaVersion := java.GetRequirement(version, p.GetCategory())
	for i := range builds {
		builds[i].Java = javaVersion
	}

	_ = s.cache.Set(ctx, cacheKey, builds)
	return builds, nil
}

// GetBuildsFiltered returns builds filtered by options
func (s *JarsService) GetBuildsFiltered(ctx context.Context, categoryID, version string, opts BuildFilterOptions) ([]models.Build, error) {
	builds, err := s.GetBuilds(ctx, categoryID, version)
	if err != nil {
		return nil, err
	}

	filtered := make([]models.Build, 0)
	for _, b := range builds {
		// Filter by stability
		if opts.StableOnly && !b.Stable {
			continue
		}

		// Filter by date range
		if opts.After != nil && !b.CreatedAt.IsZero() && b.CreatedAt.Before(*opts.After) {
			continue
		}
		if opts.Before != nil && !b.CreatedAt.IsZero() && b.CreatedAt.After(*opts.Before) {
			continue
		}

		filtered = append(filtered, b)
	}

	return filtered, nil
}

// GetBuild returns a specific build
func (s *JarsService) GetBuild(ctx context.Context, categoryID, version string, build int) (*models.Build, error) {
	p, err := s.registry.Get(categoryID)
	if err != nil {
		return nil, err
	}

	b, err := p.GetBuild(ctx, version, build)
	if err != nil {
		return nil, err
	}

	// Add Java requirement
	b.Java = java.GetRequirement(version, p.GetCategory())

	return b, nil
}

// GetLatestBuild returns the latest build for a version
func (s *JarsService) GetLatestBuild(ctx context.Context, categoryID, version string) (*models.Build, error) {
	p, err := s.registry.Get(categoryID)
	if err != nil {
		return nil, err
	}

	b, err := p.GetLatestBuild(ctx, version)
	if err != nil {
		return nil, err
	}

	// Add Java requirement
	b.Java = java.GetRequirement(version, p.GetCategory())

	return b, nil
}

// GetDownloadURL returns the download URL for a specific build
func (s *JarsService) GetDownloadURL(ctx context.Context, categoryID, version string, build int) (string, error) {
	p, err := s.registry.Get(categoryID)
	if err != nil {
		return "", err
	}

	return p.GetDownloadURL(ctx, version, build)
}

// VersionFilterOptions contains version filter parameters
type VersionFilterOptions struct {
	Type       *models.VersionType
	StableOnly bool
	Java       *int
	After      *time.Time
	Before     *time.Time
	MinYear    *int
	MaxYear    *int
}

// BuildFilterOptions contains build filter parameters
type BuildFilterOptions struct {
	StableOnly bool
	After      *time.Time
	Before     *time.Time
}

// SearchOptions contains search parameters
type SearchOptions struct {
	Query       string
	Category    *models.Category
	VersionType *models.VersionType
	Java        *int
	MinYear     *int
	MaxYear     *int
	StableOnly  bool
	After       *time.Time
	Before      *time.Time
}

// Search searches across all categories and versions
func (s *JarsService) Search(ctx context.Context, opts SearchOptions) ([]models.SearchResult, error) {
	var results []models.SearchResult

	for _, p := range s.registry.List() {
		// Filter by category
		if opts.Category != nil && p.GetCategory() != *opts.Category {
			continue
		}

		versions, err := s.GetVersions(ctx, p.GetID())
		if err != nil {
			continue
		}

		for _, v := range versions {
			// Filter by version type
			if opts.VersionType != nil && v.Type != *opts.VersionType {
				continue
			}

			// Filter by stability
			if opts.StableOnly && !v.Stable {
				continue
			}

			// Filter by Java version
			if opts.Java != nil && v.Java != *opts.Java {
				continue
			}

			// Filter by year
			if opts.MinYear != nil && !v.ReleaseTime.IsZero() && v.ReleaseTime.Year() < *opts.MinYear {
				continue
			}
			if opts.MaxYear != nil && !v.ReleaseTime.IsZero() && v.ReleaseTime.Year() > *opts.MaxYear {
				continue
			}

			// Filter by date range
			if opts.After != nil && !v.ReleaseTime.IsZero() && v.ReleaseTime.Before(*opts.After) {
				continue
			}
			if opts.Before != nil && !v.ReleaseTime.IsZero() && v.ReleaseTime.After(*opts.Before) {
				continue
			}

			// Filter by query
			if opts.Query != "" {
				query := strings.ToLower(opts.Query)
				if !strings.Contains(strings.ToLower(v.ID), query) &&
					!strings.Contains(strings.ToLower(p.GetName()), query) {
					continue
				}
			}

			results = append(results, models.SearchResult{
				Category: p.GetCategory(),
				Version:  v.ID,
				Java:     v.Java,
			})
		}
	}

	return results, nil
}

// getCategoryDescription returns a description for a category
func getCategoryDescription(cat models.Category) string {
	descriptions := map[models.Category]string{
		models.CategoryVanilla:    "Official Minecraft server from Mojang",
		models.CategoryPaper:      "High performance Minecraft server fork",
		models.CategorySpigot:     "Optimized CraftBukkit server",
		models.CategoryPurpur:     "Paper fork with extra configurability",
		models.CategoryFolia:      "Paper fork with regionized multithreading",
		models.CategoryVelocity:   "Modern, high-performance Minecraft server proxy",
		models.CategoryBungeeCord: "Minecraft server proxy by SpigotMC",
	}

	if desc, ok := descriptions[cat]; ok {
		return desc
	}
	return ""
}
