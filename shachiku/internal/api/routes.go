package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"shachiku/internal/agent"
	"shachiku/internal/config"
	"shachiku/internal/memory"
	"shachiku/internal/models"
	"shachiku/internal/provider"
	"shachiku/internal/scheduler"
	"shachiku/internal/skills"
	"shachiku/ui"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// Auth state
type AuthConfig struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}

var (
	currentAuth    *AuthConfig
	failedAttempts = make(map[string][]time.Time)
	failedMu       sync.Mutex
)

func loadAuthConfig() {
	b, err := os.ReadFile(filepath.Join(config.GetDataDir(), "auth.json"))
	if err == nil {
		var cfg AuthConfig
		if json.Unmarshal(b, &cfg) == nil {
			currentAuth = &cfg
		}
	}
}

func saveAuthConfig(cfg AuthConfig) {
	b, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(config.GetDataDir(), "auth.json"), b, 0644)
	currentAuth = &cfg
}

func checkRateLimit(ip string) bool {
	failedMu.Lock()
	defer failedMu.Unlock()
	now := time.Now()
	var recent []time.Time
	for _, t := range failedAttempts[ip] {
		if now.Sub(t) < time.Hour {
			recent = append(recent, t)
		}
	}
	failedAttempts[ip] = recent
	return len(recent) < 5
}

func recordFailedAttempt(ip string) {
	failedMu.Lock()
	defer failedMu.Unlock()
	failedAttempts[ip] = append(failedAttempts[ip], time.Now())
}

func SetupRoutes() *gin.Engine {
	r := gin.Default()
	_ = r.SetTrustedProxies(nil) // Disable proxy trust to get the true network IP

	os.MkdirAll(filepath.Join(config.GetDataDir(), "uploads"), 0755)

	// Enable CORS (basic example)
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	if os.Getenv("IS_PUBLIC") != "" {
		loadAuthConfig()

		r.Use(func(c *gin.Context) {
			path := c.Request.URL.Path
			if path == "/setup-auth" || path == "/api/setup-auth" || path == "/logo-light.json" || path == "/logo-dark.json" || path == "/favicon.svg" || strings.HasPrefix(path, "/_next/") {
				c.Next()
				return
			}

			if currentAuth == nil {
				if c.Request.Method == "GET" && !strings.HasPrefix(path, "/api/") {
					c.Redirect(http.StatusTemporaryRedirect, "/setup-auth")
					c.Abort()
					return
				}
				c.AbortWithStatusJSON(403, gin.H{"error": "Authentication setup required"})
				return
			}

			ip := c.ClientIP()
			if !checkRateLimit(ip) {
				c.String(http.StatusTooManyRequests, "Too many failed attempts. Please try again later.")
				c.Abort()
				return
			}

			user, pass, hasAuth := c.Request.BasicAuth()
			if !hasAuth {
				c.Writer.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			if user != currentAuth.Username || bcrypt.CompareHashAndPassword([]byte(currentAuth.PasswordHash), []byte(pass)) != nil {
				recordFailedAttempt(ip)
				c.Writer.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			c.Next()
		})

		r.POST("/api/setup-auth", func(c *gin.Context) {
			if currentAuth != nil {
				c.String(http.StatusBadRequest, "Already setup")
				return
			}
			username := c.PostForm("username")
			password := c.PostForm("password")
			confirm := c.PostForm("confirm_password")

			if password != confirm {
				c.String(http.StatusBadRequest, "Passwords do not match")
				return
			}

			hasUpper := regexp.MustCompile("[A-Z]").MatchString(password)
			hasLower := regexp.MustCompile("[a-z]").MatchString(password)
			hasNumber := regexp.MustCompile("[0-9]").MatchString(password)
			if !hasUpper || !hasLower || !hasNumber {
				c.String(http.StatusBadRequest, "Password must contain uppercase, lowercase and numbers")
				return
			}

			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to hash password")
				return
			}

			saveAuthConfig(AuthConfig{
				Username:     username,
				PasswordHash: string(hash),
			})

			c.String(http.StatusOK, "OK")
		})
	}

	api := r.Group("/api")
	api.Static("/uploads", filepath.Join(config.GetDataDir(), "uploads"))
	{
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "pong"})
		})

		api.POST("/chat", handleChat)
		api.GET("/config", handleGetConfig)
		api.PUT("/config", handleUpdateConfig)
		api.POST("/models", handleFetchModels)
		api.POST("/generate-soul", handleGenerateSoul)

		api.GET("/memory", handleGetMemory)
		api.DELETE("/memory", handleClearShortMemory)
		api.GET("/memory/long", handleGetLongMemory)
		api.DELETE("/memory/long/:id", handleDeleteLongMemory)

		api.GET("/skills", handleGetSkills)
		api.DELETE("/skills/:name", handleDeleteSkill)
		api.GET("/tasks", handleGetTasks)
		api.GET("/tasks/:id/logs", handleGetTaskLogs)
		api.DELETE("/tasks", handleClearTasks)
		api.DELETE("/tasks/:id", handleDeleteTask)

		api.GET("/tokens/dashboard", handleGetTokenDashboard)
	}

	// Serve the embedded Next.js UI
	distFS, err := fs.Sub(ui.Files, "dist")
	if err != nil {
		log.Println("WARNING: Failed to load embedded UI files:", err)
	} else {
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(404, gin.H{"error": "route not found"})
				return
			}

			reqPath := c.Request.URL.Path
			p := strings.TrimPrefix(reqPath, "/")
			if p == "" {
				p = "."
			}

			// Check if exact file exists
			if _, err := fs.Stat(distFS, p); err != nil {
				// Check if corresponding .html exists (Next.js logic)
				if _, err := fs.Stat(distFS, p+".html"); err == nil {
					c.Request.URL.Path = reqPath + ".html"
				} else {
					// Fallback to index.html for SPA
					c.Request.URL.Path = "/"
				}
			}

			http.FileServer(http.FS(distFS)).ServeHTTP(c.Writer, c.Request)
		})
	}

	return r
}

func handleChat(c *gin.Context) {
	var req struct {
		Message string `json:"message" form:"message"`
	}

	var filePaths []string
	var imageMarkdown []string
	uploadDir := filepath.Join(config.GetDataDir(), "uploads")
	os.MkdirAll(uploadDir, 0755)

	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(50 << 20); err == nil { // 50 MB limit
			req.Message = c.PostForm("message")

			form, _ := c.MultipartForm()
			if form != nil {
				files := form.File["files"]
				for _, file := range files {
					// Use a safe unique filename avoiding collisions
					filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(file.Filename))
					dst := filepath.Join(uploadDir, filename)
					if err := c.SaveUploadedFile(file, dst); err == nil {
						filePaths = append(filePaths, dst)

						ext := strings.ToLower(filepath.Ext(filename))
						if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" {
							scheme := "http"
							if c.Request.TLS != nil {
								scheme = "https"
							}
							baseURL := fmt.Sprintf("%s://%s/api/uploads", scheme, c.Request.Host)
							imageMarkdown = append(imageMarkdown, fmt.Sprintf("![image](%s/%s)", baseURL, filename))
						} else {
							req.Message += fmt.Sprintf("\n[System: User uploaded file locally at: %s]", dst)
						}
					}
				}
			}
		}
	} else {
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}
	}

	if len(imageMarkdown) > 0 {
		req.Message += "\n\n" + strings.Join(imageMarkdown, "\n")
	}

	log.Printf("[Chat] Received user message: %s", req.Message)

	// Set headers for SSE streaming
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	sendSSE := func(eventType, content string) {
		payload, _ := json.Marshal(map[string]string{"type": eventType, "content": content})
		c.Writer.Write([]byte("data: " + string(payload) + "\n\n"))
		c.Writer.Flush()
	}

	sendSSEError := func(errStr string) {
		payload, _ := json.Marshal(map[string]string{"error": errStr})
		c.Writer.Write([]byte("data: " + string(payload) + "\n\n"))
		c.Writer.Flush()
	}

	finalReply, err := agent.ProcessMessage(c.Request.Context(), req.Message, func(step string) {
		sendSSE("step", step)
	})

	if err != nil {
		sendSSEError(err.Error())
		return
	}

	sendSSE("result", finalReply)
}

func handleGetConfig(c *gin.Context) {
	c.JSON(200, memory.GetLLMConfig())
}

func handleUpdateConfig(c *gin.Context) {
	var req models.LLMConfig
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request format"})
		return
	}

	if err := memory.UpdateLLMConfig(req); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, req)
}

func handleFetchModels(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request format"})
		return
	}

	modelsList, err := provider.FetchModels(req.Provider, req.APIKey)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"models": modelsList})
}

func handleGetMemory(c *gin.Context) {
	c.JSON(200, memory.GetRecentHistory())
}

func handleClearShortMemory(c *gin.Context) {
	memory.ClearShortTermMemory()
	c.JSON(200, gin.H{"status": "cleared"})
}

func handleGetLongMemory(c *gin.Context) {
	facts, err := memory.GetAllLongTermMemory()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, facts)
}

func handleDeleteLongMemory(c *gin.Context) {
	id := c.Param("id")
	err := memory.DeleteFactFromLongTermMemory(id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "deleted"})
}

func handleGetSkills(c *gin.Context) {
	c.JSON(200, skills.ListSkills())
}

func handleGetTasks(c *gin.Context) {
	c.JSON(200, memory.GetTasks())
}

func handleGetTaskLogs(c *gin.Context) {
	idStr := c.Param("id")
	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(400, gin.H{"error": "Invalid task ID"})
		return
	}
	c.JSON(200, memory.GetTaskLogs(id))
}

func handleClearTasks(c *gin.Context) {
	scheduler.ClearAllTasks()
	memory.ClearTasks()
	c.JSON(200, gin.H{"status": "cleared"})
}

func handleDeleteSkill(c *gin.Context) {
	name := c.Param("name")
	if err := skills.DeleteSkill(name); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "deleted"})
}

func handleDeleteTask(c *gin.Context) {
	idStr := c.Param("id")
	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		c.JSON(400, gin.H{"error": "Invalid task ID"})
		return
	}

	scheduler.UnscheduleTask(id)

	if err := memory.DeleteTaskByID(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "deleted"})
}

func handleGetTokenDashboard(c *gin.Context) {
	metrics, err := memory.GetTokenDashboardMetrics()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, metrics)
}

func handleGenerateSoul(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Personality string `json:"personality"`
		Role        string `json:"role"`
		Language    string `json:"language"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request format"})
		return
	}

	cfg := memory.GetLLMConfig()
	soul, err := provider.GenerateSoul(c.Request.Context(), cfg, req.Name, req.Personality, req.Role, req.Language)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"soul": soul})
}
