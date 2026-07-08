package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/Rookie629/cc-switch-server/internal/preset"
	s "github.com/Rookie629/cc-switch-server/internal/service"
	"github.com/Rookie629/cc-switch-server/internal/store"
)

// NewRouter creates and configures the gin router with all API endpoints.
func NewRouter(provSvc *s.ProviderService, proxySvc *s.ProxyService, configWr *s.ConfigWriter) *gin.Engine {
	r := gin.Default()

	// CORS for local development
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api")
	{
		api.GET("/providers", func(c *gin.Context) {
			providers, err := provSvc.List()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if providers == nil {
				providers = []store.Provider{}
			}
			c.JSON(http.StatusOK, gin.H{"providers": providers})
		})

		api.GET("/providers/active", func(c *gin.Context) {
			p, err := provSvc.GetActive()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if p == nil {
				c.JSON(http.StatusOK, gin.H{"provider": nil})
				return
			}
			c.JSON(http.StatusOK, gin.H{"provider": p})
		})

		api.POST("/providers/switch/:name", func(c *gin.Context) {
			name := c.Param("name")

			p, err := provSvc.GetByName(name)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}

			// Backup
			backupPath, _ := configWr.Backup()

			// Handle proxy
			baseURL := p.BaseURL
			if p.Type != store.TypeAnthropic {
				port, err := s.SpawnProxyDaemon(p.APIKey, p.BaseURL, p.Models, p.DefaultModel)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start proxy: " + err.Error()})
					return
				}
				baseURL = "http://127.0.0.1:" + itoa(port)
			} else {
				s.KillDaemon()
			}

			extraEnv := make(map[string]string)
			if p.DefaultModel != "" {
				extraEnv["ANTHROPIC_MODEL"] = p.DefaultModel
				extraEnv["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = p.DefaultModel
				extraEnv["ANTHROPIC_DEFAULT_SONNET_MODEL"] = p.DefaultModel
				extraEnv["ANTHROPIC_DEFAULT_OPUS_MODEL"] = p.DefaultModel
			}

			if err := configWr.WriteProvider(p.APIKey, baseURL, extraEnv); err != nil {
				configWr.RestoreBackup()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write config: " + err.Error()})
				return
			}

			if _, err := provSvc.SetActive(name); err != nil {
				configWr.RestoreBackup()
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"status": "ok", "backup": backupPath, "base_url": baseURL})
		})

		api.POST("/providers", func(c *gin.Context) {
			var req struct {
				Name         string   `json:"name"`
				Type         string   `json:"type"`
				APIKey       string   `json:"api_key"`
				BaseURL      string   `json:"base_url"`
				Models       []string `json:"models"`
				DefaultModel string   `json:"default_model"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			p, err := provSvc.Add(req.Name, req.Type, req.APIKey, req.BaseURL, req.Models, req.DefaultModel)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"provider": p})
		})

		api.DELETE("/providers/:name", func(c *gin.Context) {
			name := c.Param("name")
			if err := provSvc.Remove(name); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		api.PUT("/providers/:name", func(c *gin.Context) {
			name := c.Param("name")
			var req map[string]interface{}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			p, err := provSvc.Edit(name, req)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"provider": p})
		})

		api.GET("/proxy/status", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"running": proxySvc.IsRunning(),
				"port":    proxySvc.Port(),
			})
		})

		api.GET("/presets", func(c *gin.Context) {
			presets := preset.All()
			c.JSON(http.StatusOK, gin.H{"presets": presets})
		})

		api.POST("/test-connection", func(c *gin.Context) {
			var req struct {
				APIKey  string `json:"api_key"`
				BaseURL string `json:"base_url"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// Simple test: list models or health check
			client := &http.Client{}
			httpReq, _ := http.NewRequest("GET", req.BaseURL+"/models", nil)
			httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
			resp, err := client.Do(httpReq)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
				return
			}
			defer resp.Body.Close()
			c.JSON(http.StatusOK, gin.H{"ok": resp.StatusCode < 500})
		})
	}

	// Serve embedded frontend — placeholder; during dev point to web/ dir
	r.GET("/", func(c *gin.Context) {
		c.File("web/index.html")
	})
	r.GET("/index.html", func(c *gin.Context) {
		c.File("web/index.html")
	})
	r.GET("/style.css", func(c *gin.Context) {
		c.File("web/style.css")
	})
	r.GET("/app.js", func(c *gin.Context) {
		c.File("web/app.js")
	})

	return r
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
