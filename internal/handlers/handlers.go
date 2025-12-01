package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/ServerwaveHost/wave-mc-jars-api/internal/models"
	"github.com/ServerwaveHost/wave-mc-jars-api/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler contains all HTTP handlers
type Handler struct {
	svc        *service.JarsService
	httpClient *http.Client
}

// NewHandler creates a new handler instance
func NewHandler(svc *service.JarsService) *Handler {
	return &Handler{
		svc:        svc,
		httpClient: &http.Client{},
	}
}

// APIResponse is the standard API response wrapper
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// resolveVersion resolves "latest" to the actual latest stable version ID
func (h *Handler) resolveVersion(c *gin.Context, categoryID, version string) (string, error) {
	if version == "latest" {
		latestVersion, err := h.svc.GetLatestStableVersion(c.Request.Context(), categoryID)
		if err != nil {
			return "", err
		}
		return latestVersion.ID, nil
	}
	return version, nil
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"status":  "healthy",
			"version": "1.0.0",
		},
	})
}

// GetCategories handles GET /categories
func (h *Handler) GetCategories(c *gin.Context) {
	categories := h.svc.GetCategories(c.Request.Context())
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    categories,
	})
}

// GetCategory handles GET /categories/:category
func (h *Handler) GetCategory(c *gin.Context) {
	categoryID := c.Param("category")

	category, err := h.svc.GetCategory(c.Request.Context(), categoryID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    category,
	})
}

// GetVersions handles GET /categories/:category/versions
// Query params: type, stable, supported, java, after, before, min_year, max_year
func (h *Handler) GetVersions(c *gin.Context) {
	categoryID := c.Param("category")

	opts := service.VersionFilterOptions{
		StableOnly:    c.Query("stable") == "true",
		SupportedOnly: c.Query("supported") == "true",
	}

	// Parse type filter
	if vType := c.Query("type"); vType != "" {
		versionType := models.VersionType(vType)
		opts.Type = &versionType
	}

	// Parse java filter
	if javaStr := c.Query("java"); javaStr != "" {
		if javaVersion, err := strconv.Atoi(javaStr); err == nil {
			opts.Java = &javaVersion
		}
	}

	// Parse date filters
	if afterStr := c.Query("after"); afterStr != "" {
		if t, err := time.Parse("2006-01-02", afterStr); err == nil {
			opts.After = &t
		}
	}
	if beforeStr := c.Query("before"); beforeStr != "" {
		if t, err := time.Parse("2006-01-02", beforeStr); err == nil {
			opts.Before = &t
		}
	}

	// Parse year filters
	if minYear := c.Query("min_year"); minYear != "" {
		if year, err := strconv.Atoi(minYear); err == nil {
			opts.MinYear = &year
		}
	}
	if maxYear := c.Query("max_year"); maxYear != "" {
		if year, err := strconv.Atoi(maxYear); err == nil {
			opts.MaxYear = &year
		}
	}

	versions, err := h.svc.GetVersionsFiltered(c.Request.Context(), categoryID, opts)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	response := models.VersionsResponse{
		Category: models.Category(categoryID),
		Versions: versions,
	}
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetBuilds handles GET /categories/:category/versions/:version/builds
// Query params: stable, channel, after, before
// Note: version can be "latest" to get the latest stable version
func (h *Handler) GetBuilds(c *gin.Context) {
	categoryID := c.Param("category")
	version := c.Param("version")

	// Resolve "latest" to actual version
	resolvedVersion, err := h.resolveVersion(c, categoryID, version)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	opts := service.BuildFilterOptions{
		StableOnly: c.Query("stable") == "true",
	}

	// Parse channel filter (ALPHA, BETA, STABLE, RECOMMENDED)
	if channel := c.Query("channel"); channel != "" {
		opts.Channel = &channel
	}

	// Parse date filters
	if afterStr := c.Query("after"); afterStr != "" {
		if t, err := time.Parse("2006-01-02", afterStr); err == nil {
			opts.After = &t
		}
	}
	if beforeStr := c.Query("before"); beforeStr != "" {
		if t, err := time.Parse("2006-01-02", beforeStr); err == nil {
			opts.Before = &t
		}
	}

	builds, err := h.svc.GetBuildsFiltered(c.Request.Context(), categoryID, resolvedVersion, opts)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Find latest stable (first stable since sorted newest first)
	var latestStable *models.Build
	for i := range builds {
		if builds[i].Stable {
			latestStable = &builds[i]
			break
		}
	}

	response := models.BuildsResponse{
		Category:     models.Category(categoryID),
		Version:      resolvedVersion,
		Builds:       builds,
		LatestStable: latestStable,
	}
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetBuild handles GET /categories/:category/versions/:version/builds/:build
// Note: version can be "latest" to get the latest stable version
// Note: build can be "latest" to get the latest build
func (h *Handler) GetBuild(c *gin.Context) {
	categoryID := c.Param("category")
	version := c.Param("version")
	buildStr := c.Param("build")

	// Resolve "latest" to actual version
	resolvedVersion, err := h.resolveVersion(c, categoryID, version)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	var build *models.Build

	if buildStr == "latest" {
		build, err = h.svc.GetLatestBuild(c.Request.Context(), categoryID, resolvedVersion)
	} else {
		buildNum, parseErr := strconv.Atoi(buildStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "invalid build number",
			})
			return
		}
		build, err = h.svc.GetBuild(c.Request.Context(), categoryID, resolvedVersion, buildNum)
	}

	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    build,
	})
}

// GetDownload handles GET /categories/:category/versions/:version/builds/:build/download
// This proxies the download through our API, streaming directly from source to client
// Note: version can be "latest" to get the latest stable version
// Note: build can be "latest" to get the latest build
func (h *Handler) GetDownload(c *gin.Context) {
	categoryID := c.Param("category")
	version := c.Param("version")
	buildStr := c.Param("build")

	// Resolve "latest" to actual version
	resolvedVersion, err := h.resolveVersion(c, categoryID, version)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	var build *models.Build

	if buildStr == "latest" {
		build, err = h.svc.GetLatestBuild(c.Request.Context(), categoryID, resolvedVersion)
	} else {
		buildNum, parseErr := strconv.Atoi(buildStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "invalid build number",
			})
			return
		}
		build, err = h.svc.GetBuild(c.Request.Context(), categoryID, resolvedVersion, buildNum)
	}

	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if len(build.Downloads) == 0 || build.Downloads[0].UpstreamURL == "" {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "no download available",
		})
		return
	}

	download := build.Downloads[0]

	// Create request to upstream
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", download.UpstreamURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "failed to create download request",
		})
		return
	}
	req.Header.Set("User-Agent", "jarvault/1.0.0 (https://github.com/ServerwaveHost/wave-mc-jars-api)")

	// Execute request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, APIResponse{
			Success: false,
			Error:   "failed to fetch from upstream",
		})
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("upstream returned status %d", resp.StatusCode),
		})
		return
	}

	// Determine filename
	filename := download.Name
	if filename == "" {
		filename = fmt.Sprintf("%s-%s-%d.jar", categoryID, resolvedVersion, build.Number)
	}

	// Set response headers
	c.Header("Content-Type", "application/java-archive")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// Forward Content-Length if available
	if resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}

	// Stream the response body directly to client (no disk storage)
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// Search handles GET /search
// Query params: q, category, type, stable, java, after, before, min_year, max_year
func (h *Handler) Search(c *gin.Context) {
	opts := service.SearchOptions{
		Query:      c.Query("q"),
		StableOnly: c.Query("stable") == "true",
	}

	if cat := c.Query("category"); cat != "" {
		category := models.Category(cat)
		opts.Category = &category
	}

	if vType := c.Query("type"); vType != "" {
		versionType := models.VersionType(vType)
		opts.VersionType = &versionType
	}

	// Parse java filter
	if javaStr := c.Query("java"); javaStr != "" {
		if javaVersion, err := strconv.Atoi(javaStr); err == nil {
			opts.Java = &javaVersion
		}
	}

	if minYear := c.Query("min_year"); minYear != "" {
		if year, err := strconv.Atoi(minYear); err == nil {
			opts.MinYear = &year
		}
	}

	if maxYear := c.Query("max_year"); maxYear != "" {
		if year, err := strconv.Atoi(maxYear); err == nil {
			opts.MaxYear = &year
		}
	}

	// Parse date filters
	if afterStr := c.Query("after"); afterStr != "" {
		if t, err := time.Parse("2006-01-02", afterStr); err == nil {
			opts.After = &t
		}
	}
	if beforeStr := c.Query("before"); beforeStr != "" {
		if t, err := time.Parse("2006-01-02", beforeStr); err == nil {
			opts.Before = &t
		}
	}

	results, err := h.svc.Search(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    results,
	})
}
