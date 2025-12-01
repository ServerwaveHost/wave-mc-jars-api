package java

import (
	"encoding/json"
	"os"
	"regexp"
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

	// Weekly snapshot pattern: YYwWWx (e.g., 25w46a, 24w33a)
	weeklySnapshotRegex = regexp.MustCompile(`^(\d{2})w(\d{2})[a-z]$`)
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

	lowerVersion := strings.ToLower(version)

	// Handle legacy Minecraft versions (alpha, beta, classic)
	// These are ancient versions from 2009-2011 that used Java 5/6/7
	// We'll return Java 8 as it's the oldest we reasonably support
	if strings.HasPrefix(lowerVersion, "a") || // Alpha (e.g., a1.2.6)
		strings.HasPrefix(lowerVersion, "b") || // Beta (e.g., b1.8.1)
		strings.HasPrefix(lowerVersion, "c") || // Classic (e.g., c0.30)
		strings.HasPrefix(lowerVersion, "rd-") || // Pre-classic (e.g., rd-132211)
		strings.HasPrefix(lowerVersion, "inf-") || // Infdev
		strings.Contains(lowerVersion, "indev") {
		return 8
	}

	// Handle weekly snapshots (e.g., 25w46a, 24w33a)
	// Format: YYwWWx where YY=year (20XX), WW=week, x=letter
	if matches := weeklySnapshotRegex.FindStringSubmatch(lowerVersion); matches != nil {
		year, _ := strconv.Atoi(matches[1])
		week, _ := strconv.Atoi(matches[2])

		// Map year/week to Java version based on Minecraft development timeline
		// 2024+ (year >= 24): Java 21 (1.20.5+ era)
		// 2023 (year == 23): Mostly Java 17, late 2023 Java 21
		// 2022 and earlier: Java 17 or earlier
		if year >= 24 {
			return 21
		} else if year == 23 && week >= 40 {
			// Late 2023 snapshots started requiring Java 21
			return 21
		} else if year >= 21 {
			// 2021-2023 snapshots use Java 17
			return 17
		} else if year >= 17 {
			// 2017-2020 use Java 8
			return 8
		}
		return 8
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

	// If we couldn't parse v1, it's likely a special format - return -1 to use default
	if len(parts1) == 0 {
		return -1
	}

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
	// Remove common prefixes
	version = strings.TrimPrefix(version, "v")

	// Must start with a digit to be a valid semantic version
	if len(version) == 0 || version[0] < '0' || version[0] > '9' {
		return nil
	}

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
