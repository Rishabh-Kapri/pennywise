package handler

import (
	"net/http"
	"strconv"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/gin-gonic/gin"
)

type GeocodeHandler interface {
	Search(c *gin.Context)
}

type geocodeHandler struct {
	service service.GeocodeService
}

func NewGeocodeHandler(service service.GeocodeService) GeocodeHandler {
	return &geocodeHandler{service: service}
}

// Search proxies forward geocoding so clients never call Nominatim directly.
// Going through the API keeps the 1 req/s usage policy enforced in one place
// and keeps the identifying User-Agent correct, which a browser cannot set.
func (h *geocodeHandler) Search(c *gin.Context) {
	ctx := c.Request.Context()

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))

	// Optional proximity bias. Both must parse, or the hint is simply ignored --
	// a malformed coordinate should not fail an otherwise valid search.
	var near *service.Coords
	lat, latErr := strconv.ParseFloat(c.Query("lat"), 64)
	lng, lngErr := strconv.ParseFloat(c.Query("lng"), 64)
	if latErr == nil && lngErr == nil {
		near = &service.Coords{Lat: lat, Lng: lng}
	}

	results, err := h.service.Search(ctx, query, limit, near)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []service.PlaceResult{}
	}
	c.JSON(http.StatusOK, results)
}
