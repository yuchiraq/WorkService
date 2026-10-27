package api

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"project/internal/weather"
)

var documentWeather struct {
	sync.Mutex
	config  weather.Config
	service *weather.Service
}

func weatherService() *weather.Service {
	config := weather.FromEnv()
	documentWeather.Lock()
	defer documentWeather.Unlock()
	if documentWeather.service == nil || documentWeather.config != config {
		documentWeather.config = config
		documentWeather.service = weather.New(config)
	}
	return documentWeather.service
}
func documentWeatherStatus() gin.H {
	config := weather.FromEnv()
	return gin.H{"enabled": config.Enabled(), "reason": config.Reason(), "provider": "Open-Meteo"}
}
func ConstructionCities(c *gin.Context) {
	if _, ok := constructionObject(c); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	cities, err := weatherService().Cities(ctx, c.Query("q"))
	if err != nil {
		c.JSON(503, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"cities": cities})
}
func ConstructionWeather(c *gin.Context) {
	if _, ok := constructionObject(c); !ok {
		return
	}
	lat, e := strconv.ParseFloat(c.Query("latitude"), 64)
	lon, e2 := strconv.ParseFloat(c.Query("longitude"), 64)
	city := strings.TrimSpace(c.Query("city"))
	if e != nil || e2 != nil || city == "" || len([]rune(city)) > 200 {
		c.JSON(422, gin.H{"error": "Выберите город из списка"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	days, err := weatherService().Days(ctx, city, lat, lon, c.Query("from"), c.Query("to"), time.Now())
	if err != nil {
		c.JSON(503, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"days": days, "city": city, "at": time.Now().UTC().Format(time.RFC3339)})
}
