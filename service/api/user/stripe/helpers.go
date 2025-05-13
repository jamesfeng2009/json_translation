package stripe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"json_trans_api/config"
	"json_trans_api/models/tables"
	"json_trans_api/pkg/httpclient"
	stripeService "json_trans_api/pkg/stripe"
	"net/http"
	"net/url"
	"time"
)

// getOrCreateStripeCustomer 获取或创建Stripe客户
func getOrCreateStripeCustomer(userID string) (string, error) {
	// 先检查用户是否已有Stripe客户ID
	customerID, err := getStripeCustomerID(userID)
	if err != nil {
		return "", err
	}

	if customerID != "" {
		return customerID, nil
	}

	// 获取用户信息
	user, err := getUserInfo(userID)
	if err != nil {
		return "", err
	}

	// 创建Stripe客户
	customer, err := stripeService.CreateCustomer(user.Email, user.Name)
	if err != nil {
		return "", err
	}

	// 保存客户ID到数据库
	err = saveStripeCustomerID(userID, customer.ID)
	if err != nil {
		return "", err
	}

	return customer.ID, nil
}

// getStripeCustomerID 获取用户的Stripe客户ID
func getStripeCustomerID(userID string) (string, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/users", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "stripe_customer_id")
	queryParams.Add("id", "eq."+userID)
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

	var users []struct {
		StripeCustomerID string `json:"stripe_customer_id"`
	}
	err = json.Unmarshal(bodyBytes, &users)
	if err != nil {
		return "", err
	}

	if len(users) == 0 {
		return "", nil
	}

	return users[0].StripeCustomerID, nil
}

// saveStripeCustomerID 保存Stripe客户ID到用户记录
func saveStripeCustomerID(userID, customerID string) error {
	baseURL := fmt.Sprintf("%s/rest/v1/users", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("id", "eq."+userID)
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	updateData := map[string]interface{}{
		"stripe_customer_id": customerID,
		"updated_at":         time.Now(),
	}

	updateJSON, err := json.Marshal(updateData)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH", fullURL, bytes.NewBuffer(updateJSON))
	if err != nil {
		return err
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("保存客户ID失败: %d", resp.StatusCode)
	}

	return nil
}

// getUserInfo 获取用户信息
func getUserInfo(userID string) (*tables.User, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/users", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("id", "eq."+userID)
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

	var users []tables.User
	err = json.Unmarshal(bodyBytes, &users)
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("用户不存在: %s", userID)
	}

	return &users[0], nil
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

	var plans []tables.StripePlan
	err = json.Unmarshal(bodyBytes, &plans)
	if err != nil {
		return nil, err
	}

	if len(plans) == 0 {
		return nil, fmt.Errorf("找不到计划: %s", planID)
	}

	return &plans[0], nil
}