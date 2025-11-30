package models

import "time"

// Category represents a category of Minecraft server software
type Category string

const (
	CategoryVanilla    Category = "vanilla"
	CategoryPaper      Category = "paper"
	CategorySpigot     Category = "spigot"
	CategoryPurpur     Category = "purpur"
	CategoryFolia      Category = "folia"
	CategoryVelocity   Category = "velocity"
	CategoryBungeeCord Category = "bungeecord"
)

// VersionType represents the type of version (release, snapshot, etc.)
type VersionType string

const (
	VersionTypeRelease  VersionType = "release"
	VersionTypeSnapshot VersionType = "snapshot"
	VersionTypeBeta     VersionType = "beta"
	VersionTypeAlpha    VersionType = "alpha"
)

// Version represents a Minecraft version with metadata
type Version struct {
	ID          string      `json:"id"`
	Type        VersionType `json:"type"`
	ReleaseTime time.Time   `json:"release_time,omitempty"`
	Stable      bool        `json:"stable"`
	Java        int         `json:"java,omitempty"`
}

// Build represents a specific build of server software for a version
type Build struct {
	Number    int        `json:"number"`
	Version   string     `json:"version"`
	Channel   string     `json:"channel,omitempty"`
	Stable    bool       `json:"stable"`
	CreatedAt time.Time  `json:"created_at,omitempty"`
	Downloads []Download `json:"downloads,omitempty"`
	Changes   []Change   `json:"changes,omitempty"`
	Java      int        `json:"java,omitempty"`
}

// Download represents a downloadable file (internal use - includes upstream URL)
type Download struct {
	Name        string `json:"name"`
	SHA256      string `json:"sha256,omitempty"`
	SHA1        string `json:"sha1,omitempty"`
	Size        int64  `json:"size,omitempty"`
	UpstreamURL string `json:"-"` // Hidden from JSON, internal use only
}

// Change represents a change in a build (commit, changelog entry)
type Change struct {
	Commit  string `json:"commit,omitempty"`
	Summary string `json:"summary,omitempty"`
	Author  string `json:"author,omitempty"`
}

// CategoryInfo provides information about a category
type CategoryInfo struct {
	ID          Category `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
}

// SearchFilters represents filters for searching versions
type SearchFilters struct {
	Category    *Category    `json:"category,omitempty"`
	VersionType *VersionType `json:"version_type,omitempty"`
	MinYear     *int         `json:"min_year,omitempty"`
	MaxYear     *int         `json:"max_year,omitempty"`
	Stable      *bool        `json:"stable,omitempty"`
	Query       string       `json:"query,omitempty"`
}

// SearchResult represents a search result item
type SearchResult struct {
	Category Category `json:"category"`
	Version  string   `json:"version"`
	Java     int      `json:"java,omitempty"`
}

// VersionsResponse represents the response for listing versions
type VersionsResponse struct {
	Category Category  `json:"category"`
	Versions []Version `json:"versions"`
}

// BuildsResponse represents the response for listing builds
type BuildsResponse struct {
	Category     Category `json:"category"`
	Version      string   `json:"version"`
	Builds       []Build  `json:"builds"`
	LatestStable *Build   `json:"latest_stable,omitempty"`
}
