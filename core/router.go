package core

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Frontend pages
	r.LoadHTMLGlob("frontend/*.html")
	
	base := r.Group("/nexus")
	
	base.GET("/", func(c *gin.Context) { c.HTML(200, "index.html", nil) })
	base.GET("/admin", func(c *gin.Context) { c.HTML(200, "admin_dashboard.html", nil) })
	base.GET("/admin/settings", func(c *gin.Context) { c.HTML(200, "admin_settings.html", nil) })
	base.GET("/dashboard", func(c *gin.Context) { c.HTML(200, "user_dashboard.html", nil) })
	base.GET("/user/settings", func(c *gin.Context) { c.HTML(200, "user_settings.html", nil) })

	// Public API
	api := base.Group("/api")
	api.GET("/settings", PublicSettingsHandler)
	api.POST("/auth/login", LoginHandler)
	api.POST("/auth/register", RegisterHandler)

	// User-protected routes
	user := api.Group("/user")
	user.Use(AuthMiddleware())
	{
		user.GET("/info", UserInfoHandler)
		user.PUT("/username", UpdateUsernameHandler)
		user.PUT("/password", UpdatePasswordHandler)
		user.POST("/totp/setup", SetupTOTPHandler)
		user.POST("/totp/verify", VerifyTOTPHandler)
		user.DELETE("/totp", DisableTOTPHandler)
		user.GET("/stats", GetUserStatsHandler)
		user.POST("/keys", CreateNexusKeyHandler)
		user.GET("/keys", ListNexusKeysHandler)
		user.DELETE("/keys/:id", DeleteNexusKeyHandler)
		user.POST("/upstream", CreatePrivateUpstreamKeyHandler)
		user.GET("/upstream", ListPrivateUpstreamKeysHandler)
		user.DELETE("/upstream/:id", DeletePrivateUpstreamKeyHandler)
	}

	// Admin-protected routes
	admin := api.Group("/admin")
	admin.Use(AuthMiddleware(), AdminMiddleware())
	{
		admin.GET("/stats", AdminStatsHandler)
		admin.GET("/users", ListUsersHandler)
		admin.GET("/users/:id", GetUserHandler)
		admin.PUT("/users/:id", UpdateUserHandler)
		admin.DELETE("/users/:id", DeleteUserHandler)
		admin.POST("/users/:id/reset-quotas", ResetUserQuotasHandler)
		admin.GET("/users/:id/stats", UserStatsDetailHandler)
		admin.POST("/apikeys", CreateUpstreamKeyHandler)
		admin.GET("/apikeys", ListUpstreamKeysHandler)
		admin.PUT("/apikeys/:id", UpdateUpstreamKeyHandler)
		admin.DELETE("/apikeys/:id", DeleteUpstreamKeyHandler)
		admin.GET("/nexuskeys", ListAllNexusKeysHandler)
		admin.PUT("/nexuskeys/:id", DisableNexusKeyHandler)
		admin.GET("/settings", AdminGetSettingsHandler)
		admin.PUT("/settings", AdminUpdateSettingsHandler)
	}

	// OpenAI-compatible proxy
	v1 := base.Group("/v1")
	v1.Use(ProxyAuthMiddleware())
	{
		v1.GET("/models", ListModelsHandler)
		v1.POST("/chat/completions", ProxyHandler)
	}

	return r
}
