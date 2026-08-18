package core

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"nexus-api/models"
)

// ─── Auth ────────────────────────────────────────────────────────────────────

func RegisterHandler(c *gin.Context) {
	if models.GetSetting("registration_enabled", "true") != "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Registration is disabled"})
		return
	}
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Email    string `json:"email"    binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields (username, password, email)"})
		return
	}
	var count int64
	models.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}
	models.DB.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	defTokens, _ := strconv.ParseInt(models.GetSetting("default_tokens", "100"), 10, 64)
	defAllowed := models.GetSetting("default_allowed_models", "*")

	user := models.User{
		Username:      req.Username,
		Email:         req.Email,
		Role:          "user",
		Status:        "active",
		TokensTotal:   defTokens,
		AllowedModels: defAllowed,
	}
	if err := user.SetPassword(req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	if err := models.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Registration successful"})
}

func LoginHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	var user models.User
	if err := models.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	if !user.CheckPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	if user.Status == "banned" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is banned"})
		return
	}
	if user.Status == "disabled" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
		return
	}
	if user.TotpEnabled {
		if req.Code == "" {
			c.JSON(http.StatusOK, gin.H{"require_totp": true})
			return
		}
		if !VerifyTOTP(user.TotpSecret, req.Code) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid TOTP code"})
			return
		}
	}
	token, err := GenerateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"role":  user.Role,
		"id":    user.ID,
	})
}

// ─── User Self-Service ────────────────────────────────────────────────────────

func UserInfoHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var user models.User
	if err := models.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":               user.ID,
		"username":         user.Username,
		"email":            user.Email,
		"role":             user.Role,
		"status":           user.Status,
		"totp_enabled":     user.TotpEnabled,
		"tokens_total":     user.TokensTotal,
		"tokens_used":      user.TokensUsed,
		"tokens_weekly":    user.TokensWeekly,
		"tokens_week_used": user.TokensWeekUsed,
		"tokens_monthly":   user.TokensMonthly,
		"tokens_month_used": user.TokensMonthUsed,
		"tokens_5h":        user.Tokens5h,
		"tokens_5h_used":   user.Tokens5hUsed,
		"allowed_models":   user.AllowedModels,
		"routing_enabled":  user.RoutingEnabled,
	})
}

func UpdateUsernameHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var req struct {
		Username string `json:"username" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing username"})
		return
	}
	var count int64
	models.DB.Model(&models.User{}).Where("username = ? AND id != ?", req.Username, uid).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already taken"})
		return
	}
	if err := models.DB.Model(&models.User{}).Where("id = ?", uid).Update("username", req.Username).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update username"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Username updated"})
}

func UpdatePasswordHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing fields"})
		return
	}
	var user models.User
	if err := models.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if !user.CheckPassword(req.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid current password"})
		return
	}
	if err := user.SetPassword(req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	models.DB.Save(&user)
	c.JSON(http.StatusOK, gin.H{"message": "Password updated"})
}

func SetupTOTPHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var user models.User
	if err := models.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "NexusAPI Flow",
		AccountName: user.Username,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate TOTP"})
		return
	}
	secret := key.Secret()
	models.DB.Model(&user).Update("totp_secret", secret)
	c.JSON(http.StatusOK, gin.H{
		"secret": secret,
		"uri":    key.URL(),
	})
}

func VerifyTOTPHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code"})
		return
	}
	var user models.User
	if err := models.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if !VerifyTOTP(user.TotpSecret, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid TOTP code"})
		return
	}
	models.DB.Model(&user).Update("totp_enabled", true)
	c.JSON(http.StatusOK, gin.H{"message": "2FA enabled successfully"})
}

func DisableTOTPHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing password"})
		return
	}
	var user models.User
	if err := models.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if !user.CheckPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}
	models.DB.Model(&user).Updates(map[string]interface{}{"totp_enabled": false, "totp_secret": ""})
	c.JSON(http.StatusOK, gin.H{"message": "2FA disabled"})
}

func GetUserStatsHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var stats []models.UsageStat
	models.DB.Where("user_id = ?", uid).Order("date desc, hour desc").Limit(30).Find(&stats)
	// Aggregate by model
	type ModelStat struct {
		Model      string `json:"model"`
		Provider   string `json:"provider"`
		TotalCalls int64  `json:"total_calls"`
		TotalTokens int64 `json:"total_tokens"`
	}
	agg := map[string]*ModelStat{}
	for _, s := range stats {
		if _, ok := agg[s.ModelName]; !ok {
			agg[s.ModelName] = &ModelStat{Model: s.ModelName, Provider: s.Provider}
		}
		agg[s.ModelName].TotalCalls += s.CallCount
		agg[s.ModelName].TotalTokens += s.TokensUsed
	}
	result := make([]*ModelStat, 0, len(agg))
	for _, v := range agg {
		result = append(result, v)
	}
	c.JSON(http.StatusOK, result)
}

// ─── NexusKeys (User) ────────────────────────────────────────────────────────

func CreateNexusKeyHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var req struct {
		Name string `json:"name" binding:"required"`
		Pool string `json:"pool"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing name"})
		return
	}
	if req.Pool != "private" {
		req.Pool = "global"
	}
	key := models.NexusKey{
		Key:    GenerateNexusKey(),
		UserID: uid,
		Name:   req.Name,
		Pool:   req.Pool,
		Status: "active",
	}
	if err := models.DB.Create(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create key"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":          key.ID,
		"name":        key.Name,
		"key":         key.Key,
		"key_masked":  key.Key[:12] + "..." + key.Key[len(key.Key)-4:],
		"pool":        key.Pool,
		"status":      key.Status,
		"total_calls": key.TotalCalls,
		"created_at":  key.CreatedAt,
	})
}

func ListNexusKeysHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var keys []models.NexusKey
	models.DB.Where("user_id = ?", uid).Find(&keys)
	result := make([]map[string]interface{}, 0, len(keys))
	for _, k := range keys {
		masked := k.Key
		if len(k.Key) > 16 {
			masked = k.Key[:12] + "..." + k.Key[len(k.Key)-4:]
		}
		result = append(result, map[string]interface{}{
			"id":          k.ID,
			"name":        k.Name,
			"key":         masked,
			"pool":        k.Pool,
			"status":      k.Status,
			"total_calls": k.TotalCalls,
			"last_used":   k.LastUsed,
			"created_at":  k.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, result)
}

func DeleteNexusKeyHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	id := c.Param("id")
	result := models.DB.Where("id = ? AND user_id = ?", id, uid).Delete(&models.NexusKey{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Key deleted"})
}

// ─── Private Upstream Keys (User) ────────────────────────────────────────────

func CreatePrivateUpstreamKeyHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var req struct {
		Name     string `json:"name"     binding:"required"`
		Key      string `json:"key"      binding:"required"`
		Provider string `json:"provider" binding:"required"`
		BaseURL  string `json:"base_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}
	apiKey := models.APIKey{
		Name:          req.Name,
		Key:           req.Key,
		Provider:      req.Provider,
		BaseURL:       req.BaseURL,
		UserID:        uid,
		Status:        "active",
		AllowedModels: fetchModelsFromBaseURL(req.BaseURL, req.Key),
		RoutingEnabled: true,
		Weight:        1,
	}
	if err := models.DB.Create(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create key"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":       apiKey.ID,
		"name":     apiKey.Name,
		"provider": apiKey.Provider,
		"base_url": apiKey.BaseURL,
		"status":   apiKey.Status,
	})
}

func ListPrivateUpstreamKeysHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var keys []models.APIKey
	models.DB.Where("user_id = ?", uid).Find(&keys)
	result := make([]map[string]interface{}, 0, len(keys))
	for _, k := range keys {
		result = append(result, map[string]interface{}{
			"id":           k.ID,
			"name":         k.Name,
			"provider":     k.Provider,
			"base_url":     k.BaseURL,
			"status":       k.Status,
			"weight":       k.Weight,
			"total_calls":  k.TotalCalls,
			"total_tokens": k.TotalTokens,
			"error_count":  k.ErrorCount,
		})
	}
	c.JSON(http.StatusOK, result)
}

func DeletePrivateUpstreamKeyHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	id := c.Param("id")
	result := models.DB.Where("id = ? AND user_id = ?", id, uid).Delete(&models.APIKey{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Key deleted"})
}

// ─── Admin – Users ────────────────────────────────────────────────────────────

func ListUsersHandler(c *gin.Context) {
	search := c.Query("search")
	var users []models.User
	q := models.DB.Model(&models.User{})
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("username LIKE ? OR email LIKE ?", like, like)
	}
	q.Find(&users)
	result := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		result = append(result, map[string]interface{}{
			"id":                u.ID,
			"username":          u.Username,
			"email":             u.Email,
			"role":              u.Role,
			"status":            u.Status,
			"tokens_total":      u.TokensTotal,
			"tokens_used":       u.TokensUsed,
			"tokens_weekly":     u.TokensWeekly,
			"tokens_week_used":  u.TokensWeekUsed,
			"tokens_monthly":    u.TokensMonthly,
			"tokens_month_used": u.TokensMonthUsed,
			"tokens_5h":         u.Tokens5h,
			"tokens_5h_used":    u.Tokens5hUsed,
			"allowed_models":    u.AllowedModels,
			"routing_enabled":   u.RoutingEnabled,
			"totp_enabled":      u.TotpEnabled,
			"created_at":        u.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, result)
}

func GetUserHandler(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := models.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{
		"id":                user.ID,
		"username":          user.Username,
		"email":             user.Email,
		"role":              user.Role,
		"status":            user.Status,
		"tokens_total":      user.TokensTotal,
		"tokens_used":       user.TokensUsed,
		"tokens_weekly":     user.TokensWeekly,
		"tokens_week_used":  user.TokensWeekUsed,
		"tokens_monthly":    user.TokensMonthly,
		"tokens_month_used": user.TokensMonthUsed,
		"tokens_5h":         user.Tokens5h,
		"tokens_5h_used":    user.Tokens5hUsed,
		"allowed_models":    user.AllowedModels,
		"routing_enabled":   user.RoutingEnabled,
	})
}

func UpdateUserHandler(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := models.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	var req struct {
		Status          *string `json:"status"`
		Role            *string `json:"role"`
		TokensTotal     *int64  `json:"tokens_total"`
		TokensWeekly    *int64  `json:"tokens_weekly"`
		TokensMonthly   *int64  `json:"tokens_monthly"`
		Tokens5h        *int64  `json:"tokens_5h"`
		AllowedModels   *string `json:"allowed_models"`
		RoutingEnabled  *bool   `json:"routing_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	updates := map[string]interface{}{}
	if req.Status != nil         { updates["status"] = *req.Status }
	if req.Role != nil           { updates["role"] = *req.Role }
	if req.TokensTotal != nil    { updates["tokens_total"] = *req.TokensTotal }
	if req.TokensWeekly != nil   { updates["tokens_weekly"] = *req.TokensWeekly }
	if req.TokensMonthly != nil  { updates["tokens_monthly"] = *req.TokensMonthly }
	if req.Tokens5h != nil       { updates["tokens5h"] = *req.Tokens5h }
	if req.AllowedModels != nil  { updates["allowed_models"] = *req.AllowedModels }
	if req.RoutingEnabled != nil { updates["routing_enabled"] = *req.RoutingEnabled }
	models.DB.Model(&user).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"message": "User updated"})
}

func DeleteUserHandler(c *gin.Context) {
	id := c.Param("id")
	// Prevent deleting admin account
	var user models.User
	if err := models.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.Role == "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete admin account"})
		return
	}
	models.DB.Delete(&models.User{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

func ResetUserQuotasHandler(c *gin.Context) {
	id := c.Param("id")
	models.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"tokens_week_used":   0,
		"tokens_week_reset":  time.Now().Unix(),
		"tokens_month_used":  0,
		"tokens_month_reset": time.Now().Unix(),
		"tokens5h_used":      0,
		"tokens5h_reset":     0,
	})
	c.JSON(http.StatusOK, gin.H{"message": "Quotas reset"})
}

func UserStatsDetailHandler(c *gin.Context) {
	id := c.Param("id")
	var stats []models.UsageStat
	models.DB.Where("user_id = ?", id).Order("date desc, hour desc").Find(&stats)
	c.JSON(http.StatusOK, stats)
}

// ─── Admin – Global API Keys ──────────────────────────────────────────────────

func CreateUpstreamKeyHandler(c *gin.Context) {
	var req struct {
		Name           string `json:"name"            binding:"required"`
		Key            string `json:"key"             binding:"required"`
		Provider       string `json:"provider"        binding:"required"`
		BaseURL        string `json:"base_url"`
		Weight         int    `json:"weight"`
		AllowedModels  string `json:"allowed_models"`
		RoutingEnabled *bool  `json:"routing_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}
	if req.Weight <= 0 {
		req.Weight = 1
	}
	if req.AllowedModels == "" || req.AllowedModels == "*" {
		req.AllowedModels = fetchModelsFromBaseURL(req.BaseURL, req.Key)
	}
	re := true
	if req.RoutingEnabled != nil {
		re = *req.RoutingEnabled
	}
	apiKey := models.APIKey{
		Name:           req.Name,
		Key:            req.Key,
		Provider:       req.Provider,
		BaseURL:        req.BaseURL,
		UserID:         0,
		Weight:         req.Weight,
		Status:         "active",
		AllowedModels:  req.AllowedModels,
		RoutingEnabled: re,
	}
	if err := models.DB.Create(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create key"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Key added", "id": apiKey.ID})
}

func ListUpstreamKeysHandler(c *gin.Context) {
	search := c.Query("search")
	var keys []models.APIKey
	q := models.DB.Where("user_id = ?", 0)
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("name LIKE ? OR provider LIKE ?", like, like)
	}
	q.Find(&keys)
	result := make([]map[string]interface{}, 0, len(keys))
	for _, k := range keys {
		masked := k.Key
		if len(k.Key) > 12 {
			masked = k.Key[:8] + "..." + k.Key[len(k.Key)-4:]
		}
		result = append(result, map[string]interface{}{
			"id":              k.ID,
			"name":            k.Name,
			"key_masked":      masked,
			"provider":        k.Provider,
			"base_url":        k.BaseURL,
			"weight":          k.Weight,
			"status":          k.Status,
			"allowed_models":  k.AllowedModels,
			"routing_enabled": k.RoutingEnabled,
			"total_calls":     k.TotalCalls,
			"total_tokens":    k.TotalTokens,
			"error_count":     k.ErrorCount,
			"last_used":       k.LastUsed,
		})
	}
	c.JSON(http.StatusOK, result)
}

func UpdateUpstreamKeyHandler(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name           *string `json:"name"`
		Provider       *string `json:"provider"`
		BaseURL        *string `json:"base_url"`
		Weight         *int    `json:"weight"`
		Status         *string `json:"status"`
		AllowedModels  *string `json:"allowed_models"`
		RoutingEnabled *bool   `json:"routing_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil           { updates["name"] = *req.Name }
	if req.Provider != nil       { updates["provider"] = *req.Provider }
	if req.BaseURL != nil        { updates["base_url"] = *req.BaseURL }
	if req.Weight != nil         { updates["weight"] = *req.Weight }
	if req.Status != nil         { updates["status"] = *req.Status }
	if req.AllowedModels != nil {
		if *req.AllowedModels == "" || *req.AllowedModels == "*" {
			var k models.APIKey
			if err := models.DB.First(&k, id).Error; err == nil {
				bu := k.BaseURL
				if req.BaseURL != nil {
					bu = *req.BaseURL
				}
				updates["allowed_models"] = fetchModelsFromBaseURL(bu, k.Key)
			}
		} else {
			updates["allowed_models"] = *req.AllowedModels
		}
	}
	if req.RoutingEnabled != nil { updates["routing_enabled"] = *req.RoutingEnabled }
	if err := models.DB.Model(&models.APIKey{}).Where("id = ? AND user_id = ?", id, 0).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Key updated"})
}

func DeleteUpstreamKeyHandler(c *gin.Context) {
	id := c.Param("id")
	models.DB.Where("id = ? AND user_id = ?", id, 0).Delete(&models.APIKey{})
	c.JSON(http.StatusOK, gin.H{"message": "Key deleted"})
}

// ─── Admin – NexusKeys ───────────────────────────────────────────────────────

func ListAllNexusKeysHandler(c *gin.Context) {
	search := c.Query("search")
	var keys []models.NexusKey
	q := models.DB
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("name LIKE ?", like)
	}
	q.Find(&keys)

	// fetch usernames
	userIDs := make([]uint, 0, len(keys))
	for _, k := range keys {
		userIDs = append(userIDs, k.UserID)
	}
	var users []models.User
	models.DB.Where("id IN ?", userIDs).Find(&users)
	userMap := map[uint]string{}
	for _, u := range users {
		userMap[u.ID] = u.Username
	}

	result := make([]map[string]interface{}, 0, len(keys))
	for _, k := range keys {
		masked := k.Key
		if len(k.Key) > 16 {
			masked = k.Key[:12] + "..." + k.Key[len(k.Key)-4:]
		}
		result = append(result, map[string]interface{}{
			"id":          k.ID,
			"name":        k.Name,
			"key":         masked,
			"user_id":     k.UserID,
			"username":    userMap[k.UserID],
			"pool":        k.Pool,
			"status":      k.Status,
			"total_calls": k.TotalCalls,
			"last_used":   k.LastUsed,
		})
	}
	c.JSON(http.StatusOK, result)
}

func DisableNexusKeyHandler(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required,oneof=active disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status must be 'active' or 'disabled'"})
		return
	}
	models.DB.Model(&models.NexusKey{}).Where("id = ?", id).Update("status", req.Status)
	c.JSON(http.StatusOK, gin.H{"message": "Status updated"})
}

// ─── Admin – Settings ────────────────────────────────────────────────────────

func AdminGetSettingsHandler(c *gin.Context) {
	keys := []string{
		"announcement", "default_tokens", "easter_egg_message", "easter_egg_models",
		"registration_enabled", "default_allowed_models",
	}
	result := map[string]string{}
	for _, k := range keys {
		result[k] = models.GetSetting(k, "")
	}
	c.JSON(http.StatusOK, result)
}

func AdminUpdateSettingsHandler(c *gin.Context) {
	allowed := map[string]bool{
		"announcement": true, "default_tokens": true, "easter_egg_message": true,
		"easter_egg_models": true, "registration_enabled": true, "default_allowed_models": true,
	}
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	for k, v := range req {
		if allowed[k] {
			models.SetSetting(k, v)
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "Settings updated"})
}

// ─── Admin – Stats ────────────────────────────────────────────────────────────

func AdminStatsHandler(c *gin.Context) {
	var totalUsers, totalActiveUsers int64
	models.DB.Model(&models.User{}).Count(&totalUsers)
	models.DB.Model(&models.User{}).Where("status = ?", "active").Count(&totalActiveUsers)

	var totalKeys int64
	models.DB.Model(&models.APIKey{}).Where("user_id = 0 AND status = ?", "active").Count(&totalKeys)

	// Total calls and tokens from usage stats
	type TotalUsage struct {
		TotalCalls  int64
		TotalTokens int64
	}
	var usage TotalUsage
	models.DB.Model(&models.UsageStat{}).
		Select("SUM(call_count) as total_calls, SUM(tokens_used) as total_tokens").
		Scan(&usage)

	// Top 10 users by tokens used
	type UserStat struct {
		UserID      uint   `json:"user_id"`
		Username    string `json:"username"`
		TotalCalls  int64  `json:"total_calls"`
		TotalTokens int64  `json:"total_tokens"`
	}
	var topUsers []UserStat
	models.DB.Model(&models.UsageStat{}).
		Select("user_id, SUM(call_count) as total_calls, SUM(tokens_used) as total_tokens").
		Group("user_id").
		Order("total_tokens desc").
		Limit(10).
		Scan(&topUsers)
	// Enrich with usernames
	for i := range topUsers {
		var u models.User
		if err := models.DB.First(&u, topUsers[i].UserID).Error; err == nil {
			topUsers[i].Username = u.Username
		}
	}

	// Top 10 models
	type ModelStat struct {
		Model       string `json:"model"`
		Provider    string `json:"provider"`
		TotalCalls  int64  `json:"total_calls"`
		TotalTokens int64  `json:"total_tokens"`
	}
	var topModels []ModelStat
	models.DB.Model(&models.UsageStat{}).
		Select("model_name as model, provider, SUM(call_count) as total_calls, SUM(tokens_used) as total_tokens").
		Group("model_name").
		Order("total_calls desc").
		Limit(10).
		Scan(&topModels)

	c.JSON(http.StatusOK, gin.H{
		"total_users":        totalUsers,
		"total_active_users": totalActiveUsers,
		"total_active_keys":  totalKeys,
		"total_calls":        usage.TotalCalls,
		"total_tokens":       usage.TotalTokens,
		"top_users":          topUsers,
		"top_models":         topModels,
	})
}

// ─── Public ──────────────────────────────────────────────────────────────────

func PublicSettingsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"announcement":         models.GetSetting("announcement", ""),
		"registration_enabled": models.GetSetting("registration_enabled", "true"),
	})
}

// ListModelsHandler returns OpenAI-compatible model list
func ListModelsHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	var user models.User
	models.DB.First(&user, uid)

	now := time.Now().Unix()
	type ModelInfo struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	var data []ModelInfo

	// Build from global API keys
	var keys []models.APIKey
	models.DB.Where("user_id = 0 AND status = ?", "active").Find(&keys)

	seen := map[string]bool{}
	// Common model lists per provider
	providerModels := map[string][]string{
		"openai":    {"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4", "gpt-3.5-turbo", "o1", "o1-mini", "o3-mini"},
		"anthropic": {"claude-opus-4-5", "claude-sonnet-4-5", "claude-haiku-3-5", "claude-3-opus-20240229", "claude-3-5-sonnet-20241022", "claude-3-haiku-20240307"},
		"gemini":    {"gemini-2.5-pro", "gemini-2.0-flash", "gemini-1.5-pro", "gemini-1.5-flash"},
	}

	for _, k := range keys {
		if k.AllowedModels == "*" {
			if models, ok := providerModels[k.Provider]; ok {
				for _, m := range models {
					if !seen[m] {
						seen[m] = true
						data = append(data, ModelInfo{ID: m, Object: "model", Created: now, OwnedBy: k.Provider})
					}
				}
			}
		} else {
			for _, m := range strings.Split(k.AllowedModels, ",") {
				m = strings.TrimSpace(m)
				if m != "" && !seen[m] {
					seen[m] = true
					data = append(data, ModelInfo{ID: m, Object: "model", Created: now, OwnedBy: k.Provider})
				}
			}
		}
	}

	// Easter egg models
	eggModels := models.GetSetting("easter_egg_models", "free_llm_chat")
	for _, m := range strings.Split(eggModels, ",") {
		m = strings.TrimSpace(m)
		if m != "" && !seen[m] {
			seen[m] = true
			data = append(data, ModelInfo{ID: m, Object: "model", Created: now, OwnedBy: "nexus-easter-egg"})
		}
	}

	_ = json.Marshal // suppress unused import warning
	_ = user

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}


// fetchModelsFromBaseURL attempts to fetch models from the /v1/models endpoint
func fetchModelsFromBaseURL(baseURL, apiKey string) string {
	if baseURL == "" {
		return "*"
	}
	url := strings.TrimRight(baseURL, "/") + "/models"
	if !strings.HasSuffix(baseURL, "/v1") {
		url = strings.TrimRight(baseURL, "/") + "/v1/models"
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "*"
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "*"
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return "*"
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "*"
	}
	var res struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Data) == 0 {
		return "*"
	}
	
	var modelsList []string
	for _, m := range res.Data {
		modelsList = append(modelsList, m.ID)
	}
	return strings.Join(modelsList, ",")
}
