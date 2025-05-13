package quota

import (
	"encoding/json"
	"fmt"
	"io"
	"json_trans_api/config"
	"json_trans_api/models/models"
	"json_trans_api/models/tables"
	"json_trans_api/pkg/httpclient"
	responsex "json_trans_api/pkg/response"
	"json_trans_api/service/api/middleware/auth"
	"log"
	"net/http"
	"net/url"
	"time"
)

// CheckQuota 检查用户配额的中间件
func CheckQuota(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 获取用户ID
		userID := auth.GetUserIDFromContext(r)
		if userID == "" {
			// 如果没有用户ID，可能是使用API Key的请求
			// 从API Key获取用户ID
			apiKey := r.Header.Get("jt-api-key")
			if apiKey != "" {
				var err error
				userID, err = getUserIDFromAPIKey(apiKey)
				if err != nil {
					log.Printf("从API Key获取用户ID失败: %v", err)
					responsex.RespondWithJSON(w, http.StatusUnauthorized, models.Response{
						Code: http.StatusUnauthorized,
						Msg:  "无效的API Key",
						Data: map[string]interface{}{},
					})
					return
				}
			} else {
				// 如果既没有用户ID也没有API Key，则拒绝请求
				responsex.RespondWithJSON(w, http.StatusUnauthorized, models.Response{
					Code: http.StatusUnauthorized,
					Msg:  "未授权的请求",
					Data: map[string]interface{}{},
				})
				return
			}
		}

		// 获取用户当前的订阅计划和使用情况
		subscription, err := getCurrentSubscription(userID)
		if err != nil {
			log.Printf("获取用户订阅失败: %v", err)
			responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
				Code: http.StatusInternalServerError,
				Msg:  "内部服务器错误",
				Data: map[string]interface{}{},
			})
			return
		}

		// 如果用户没有活跃的订阅，使用免费计划限制
		if subscription == nil {
			// 获取免费计划的限制
			freeLimit := 1000 // 默认免费计划字符限制
			usage, err := getCurrentMonthUsage(userID)
			if err != nil {
				log.Printf("获取用户使用情况失败: %v", err)
				responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
					Code: http.StatusInternalServerError,
					Msg:  "内部服务器错误",
					Data: map[string]interface{}{},
				})
				return
			}

			// 检查是否超过免费限制
			if usage >= freeLimit {
				responsex.RespondWithJSON(w, http.StatusForbidden, models.Response{
					Code: http.StatusForbidden,
					Msg:  "您已达到免费计划的使用限制，请升级订阅计划",
					Data: map[string]interface{}{
						"current_usage": usage,
						"limit":         freeLimit,
					},
				})
				return
			}
		} else {
			// 检查订阅状态
			if subscription.Status != "active" && subscription.Status != "trialing" {
				responsex.RespondWithJSON(w, http.StatusForbidden, models.Response{
					Code: http.StatusForbidden,
					Msg:  "您的订阅不是活跃状态，请更新您的支付信息",
					Data: map[string]interface{}{
						"subscription_status": subscription.Status,
					},
				})
				return
			}

			// 检查是否超过订阅计划的限制
			usage, err := getCurrentMonthUsage(userID)
			if err != nil {
				log.Printf("获取用户使用情况失败: %v", err)
				responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
					Code: http.StatusInternalServerError,
					Msg:  "内部服务器错误",
					Data: map[string]interface{}{},
				})
				return
			}

			if usage >= subscription.CharacterLimit {
				responsex.RespondWithJSON(w, http.StatusForbidden, models.Response{
					Code: http.StatusForbidden,
					Msg:  "您已达到当前订阅计划的使用限制，请升级订阅计划",
					Data: map[string]interface{}{
						"current_usage": usage,
						"limit":         subscription.CharacterLimit,
					},
				})
				return
			}
		}

		// 继续处理请求
		next.ServeHTTP(w, r)
	})
}

// 订阅信息
type Subscription struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	SubscriptionID string    `json:"subscription_id"`
	PlanID         string    `json:"plan_id"`
	Status         string    `json:"status"`
	CharacterLimit int       `json:"character_limit"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// getCurrentSubscription 获取用户当前的订阅
func getCurrentSubscription(userID string) (*Subscription, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("status", "eq.active")
	queryParams.Add("order", "created_at.desc")
	queryParams.Add("limit", "1")
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var subscriptions []Subscription
	err = json.Unmarshal(bodyBytes, &subscriptions)
	if err != nil {
		return nil, err
	}

	if len(subscriptions) == 0 {
		return nil, nil
	}

	return &subscriptions[0], nil
}

// getCurrentMonthUsage 获取用户当前月的使用量
func getCurrentMonthUsage(userID string) (int, error) {
	// 获取当前月的开始和结束时间
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Second)

	baseURL := fmt.Sprintf("%s/rest/v1/usage_records", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "sum(character_count)")
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("created_at", "gte."+startOfMonth.Format(time.RFC3339))
	queryParams.Add("created_at", "lte."+endOfMonth.Format(time.RFC3339))
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result []struct {
		Sum int `json:"sum"`
	}
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return 0, err
	}

	if len(result) == 0 || result[0].Sum == 0 {
		return 0, nil
	}

	return result[0].Sum, nil
}

// getUserIDFromAPIKey 从API Key获取用户ID
func getUserIDFromAPIKey(apiKey string) (string, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/api_keys", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "user_id")
	queryParams.Add("key", "eq."+apiKey)
	queryParams.Add("limit", "1")
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var apiKeys []struct {
		UserID string `json:"user_id"`
	}
	err = json.Unmarshal(bodyBytes, &apiKeys)
	if err != nil {
		return "", err
	}

	if len(apiKeys) == 0 {
		return "", fmt.Errorf("无效的API Key")
	}

	return apiKeys[0].UserID, nil
}

// getCurrentPlan 获取用户当前订阅计划
func getCurrentPlan(userID string) (*tables.StripePlan, error) {
	// 从数据库获取用户的订阅信息
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("status", "eq.active")
	queryParams.Add("order", "created_at.desc")
	queryParams.Add("limit", "1")
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return getFreePlan(), err
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return getFreePlan(), err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return getFreePlan(), err
	}

	var subscriptions []tables.StripeSubscription
	err = json.Unmarshal(bodyBytes, &subscriptions)
	if err != nil {
		return getFreePlan(), err
	}

	if len(subscriptions) == 0 {
		// 用户没有活跃订阅，返回免费计划
		return getFreePlan(), nil
	}

	// 获取计划详情
	subscription := subscriptions[0]
	return getPlanDetails(subscription.PlanID)
}

// getFreePlan 获取免费计划
func getFreePlan() *tables.StripePlan {
	return &tables.StripePlan{
		Name:           "免费计划",
		Amount:         0,
		Currency:       "usd",
		Interval:       "month",
		CharacterLimit: 10000, // 免费计划每月10000字符
	}
}

// getPlanDetails 获取计划详情
func getPlanDetails(planID string) (*tables.StripePlan, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_plans", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("plan_id", "eq."+planID)
	queryParams.Add("limit", "1")
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return getFreePlan(), err
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return getFreePlan(), err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return getFreePlan(), err
	}

	var plans []tables.StripePlan
	err = json.Unmarshal(bodyBytes, &plans)
	if err != nil {
		return getFreePlan(), err
	}

	if len(plans) == 0 {
		return getFreePlan(), fmt.Errorf("找不到计划: %s", planID)
	}

	return &plans[0], nil
}

// getCurrentMonthUsage 获取用户当月使用量
func getCurrentMonthUsage(userID string) (int, error) {
	// 获取当前月份的开始和结束时间
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Second)

	// 从数据库获取用户的使用记录
	baseURL := fmt.Sprintf("%s/rest/v1/usage_records", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "characters")
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("created_at", "gte."+startOfMonth.Format(time.RFC3339))
	queryParams.Add("created_at", "lte."+endOfMonth.Format(time.RFC3339))
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var usageRecords []struct {
		Characters int `json:"characters"`
	}
	err = json.Unmarshal(bodyBytes, &usageRecords)
	if err != nil {
		return 0, err
	}

	// 计算总使用量
	totalUsage := 0
	for _, record := range usageRecords {
		totalUsage += record.Characters
	}

	return totalUsage, nil
}