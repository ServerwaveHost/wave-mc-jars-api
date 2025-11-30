package java

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ServerwaveHost/wave-mc-jars-api/internal/models"
)

// JavaConfig represents the Java version configuration
type JavaConfig struct {
	Servers []VersionRequirement `json:"servers"`
	Proxies []VersionRequirement `json:"proxies"`
	Default int                  `json:"default"`
}

// VersionRequirement represents a minimum version and its Java requirement
type VersionRequirement struct {
	MinVersion string `json:"min_version"`
	Java       int    `json:"java"`
}

var (
	config     *JavaConfig
	configOnce sync.Once
	configErr  error
)

// loadConfig loads the Java configuration from file
func loadConfig() (*JavaConfig, error) {
	configOnce.Do(func() {
		path := os.Getenv("JAVA_CONFIG_PATH")
		if path == "" {
			path = "java.json"
		}

		data, err := os.ReadFile(path)
		if err != nil {
			configErr = err
			return
		}

		config = &JavaConfig{}
		configErr = json.Unmarshal(data, config)
	})

	return config, configErr
}

// GetRequirement returns Java version requirement for a Minecraft version
func GetRequirement(version string, category models.Category) int {
	cfg, err := loadConfig()
	if err != nil {
		return 17 // Safe default
	}

	// Determine which requirements to use
	var requirements []VersionRequirement
	if isProxy(category) {
		requirements = cfg.Proxies
	} else {
		requirements = cfg.Servers
	}

	// Find matching requirement
	for _, req := range requirements {
		if compareVersions(version, req.MinVersion) >= 0 {
			return req.Java
		}
	}

	return cfg.Default
}

// isProxy returns true if the category is a proxy server
func isProxy(category models.Category) bool {
	return category == models.CategoryVelocity ||
		category == models.CategoryBungeeCord
}

// compareVersions compares two version strings
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	// Handle special cases
	if v1 == "latest" {
		return 1
	}
	if v2 == "0" {
		return 1
	}

	parts1 := parseVersionParts(v1)
	parts2 := parseVersionParts(v2)

	// Compare each part
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		p1 := 0
		p2 := 0

		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 > p2 {
			return 1
		}
		if p1 < p2 {
			return -1
		}
	}

	return 0
}

// parseVersionParts extracts numeric parts from a version string
func parseVersionParts(version string) []int {
	// Remove common prefixes and suffixes
	version = strings.TrimPrefix(version, "v")

	// Split by dots
	parts := strings.Split(version, ".")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		// Handle cases like "1.21-pre1" or "1.21.1-rc1"
		numStr := strings.Split(part, "-")[0]
		num, err := strconv.Atoi(numStr)
		if err == nil {
			result = append(result, num)
		}
	}

	return result
}
