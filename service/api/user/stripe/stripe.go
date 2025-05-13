package stripe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"json_trans_api/config"
	"json_trans_api/models/models"
	"json_trans_api/models/tables"
	"json_trans_api/pkg/httpclient"
	responsex "json_trans_api/pkg/response"
	stripeService "json_trans_api/pkg/stripe"
	"json_trans_api/service/api/middleware/auth"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/stripe/stripe-go/v72"
)

// 初始化Stripe
func init() {
	stripeService.Init()
}

// 计划信息
type PlanInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Price          int    `json:"price"`
	Currency       string `json:"currency"`
	Interval       string `json:"interval"`
	CharacterLimit int    `json:"character_limit"`
	Support        string `json:"support"`
	Webhook        bool   `json:"webhook"`
}

// 订阅信息
type SubscriptionInfo struct {
	ID                string    `json:"id"`
	Status            string    `json:"status"`
	CurrentPeriodEnd  string    `json:"current_period_end"`
	CancelAtPeriodEnd bool      `json:"cancel_at_period_end"`
	Plan              PlanInfo  `json:"plan"`
	CreatedAt         time.Time `json:"created_at"`
}

// 支付方式信息
type PaymentMethodInfo struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Last4     string `json:"last4"`
	Brand     string `json:"brand"`
	ExpMonth  int    `json:"exp_month"`
	ExpYear   int    `json:"exp_year"`
	IsDefault bool   `json:"is_default"`
}

// 发票信息
type InvoiceInfo struct {
	ID            string    `json:"id"`
	Amount        int       `json:"amount"`
	Currency      string    `json:"currency"`
	Status        string    `json:"status"`
	InvoiceURL    string    `json:"invoice_url"`
	InvoicePDF    string    `json:"invoice_pdf"`
	CreatedAt     time.Time `json:"created_at"`
}

// GetPlans 获取所有可用计划
func GetPlans(w http.ResponseWriter, r *http.Request) {
	// 从数据库获取所有计划
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_plans", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	var plans []tables.StripePlan
	err = json.Unmarshal(bodyBytes, &plans)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	// 转换为前端所需格式
	var planInfos []PlanInfo
	for _, plan := range plans {
		planInfos = append(planInfos, PlanInfo{
			ID:             plan.PlanID,
			Name:           plan.Name,
			Price:          plan.Amount / 100, // 转换为元
			Currency:       plan.Currency,
			Interval:       plan.Interval,
			CharacterLimit: plan.CharacterLimit,
			Support:        "24/7", // 可以从plan.Features中获取
			Webhook:        true,   // 可以从plan.Features中获取
		})
	}

	responsex.RespondWithJSON(w, http.StatusOK, models.Response{
		Code: http.StatusOK,
		Msg:  "成功",
		Data: planInfos,
	})
}

// CreateCheckoutSession 创建结账会话
func CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID     string `json:"plan_id"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsex.RespondWithJSON(w, http.StatusBadRequest, models.Response{
			Code: http.StatusBadRequest,
			Msg:  "无效的请求格式",
			Data: map[string]interface{}{},
		})
		return
	}

	// 获取用户信息
	userID := auth.GetUserIDFromContext(r)
	
	// 获取或创建Stripe客户
	customerID, err := getOrCreateStripeCustomer(userID)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "创建客户失败",
			Data: map[string]interface{}{},
		})
		return
	}

	// 创建结账会话
	session, err := stripeService.CreateCheckoutSession(customerID, req.PlanID, req.SuccessURL, req.CancelURL)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "创建结账会话失败",
			Data: map[string]interface{}{},
		})
		return
	}

	responsex.RespondWithJSON(w, http.StatusOK, models.Response{
		Code: http.StatusOK,
		Msg:  "成功",
		Data: map[string]interface{}{
			"session_id": session.ID,
			"url":        session.URL,
		},
	})
}

// GetCurrentSubscription 获取当前订阅
func GetCurrentSubscription(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r)
	
	// 从数据库获取用户的订阅信息
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("order", "created_at.desc")
	queryParams.Add("limit", "1")
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	var subscriptions []tables.StripeSubscription
	err = json.Unmarshal(bodyBytes, &subscriptions)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	if len(subscriptions) == 0 {
		// 用户没有订阅，返回免费计划
		responsex.RespondWithJSON(w, http.StatusOK, models.Response{
			Code: http.StatusOK,
			Msg:  "成功",
			Data: map[string]interface{}{
				"plan": PlanInfo{
					Name:           "免费计划",
					Price:          0,
					Currency:       "usd",
					Interval:       "month",
					CharacterLimit: 10000,
					Support:        "社区支持",
					Webhook:        false,
				},
			},
		})
		return
	}

	// 获取计划详情
	subscription := subscriptions[0]
	plan, err := getPlanDetails(subscription.PlanID)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	// 构建响应
	subscriptionInfo := SubscriptionInfo{
		ID:                subscription.SubscriptionID,
		Status:            subscription.Status,
		CurrentPeriodEnd:  subscription.CurrentPeriodEnd,
		CancelAtPeriodEnd: subscription.CancelAtPeriodEnd,
		Plan: PlanInfo{
			ID:             plan.PlanID,
			Name:           plan.Name,
			Price:          plan.Amount / 100,
			Currency:       plan.Currency,
			Interval:       plan.Interval,
			CharacterLimit: plan.CharacterLimit,
			Support:        "24/7",
			Webhook:        true,
		},
		CreatedAt: subscription.CreatedAt,
	}

	responsex.RespondWithJSON(w, http.StatusOK, models.Response{
		Code: http.StatusOK,
		Msg:  "成功",
		Data: subscriptionInfo,
	})
}

// CancelSubscription 取消订阅
func CancelSubscription(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r)
	
	// 获取当前订阅
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("status", "eq.active")
	queryParams.Add("limit", "1")
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	var subscriptions []tables.StripeSubscription
	err = json.Unmarshal(bodyBytes, &subscriptions)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	if len(subscriptions) == 0 {
		responsex.RespondWithJSON(w, http.StatusBadRequest, models.Response{
			Code: http.StatusBadRequest,
			Msg:  "没有找到活跃的订阅",
			Data: map[string]interface{}{},
		})
		return
	}

	// 取消订阅
	subscription := subscriptions[0]
	_, err = stripeService.CancelSubscription(subscription.SubscriptionID, true)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "取消订阅失败",
			Data: map[string]interface{}{},
		})
		return
	}

	// 更新数据库中的订阅状态
	updateURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions?id=eq.%s", config.Cfg.Supabase.SupabaseUrl, subscription.ID)
	updateData := map[string]interface{}{
		"cancel_at_period_end": true,
		"updated_at":           time.Now(),
	}
	
	updateJSON, _ := json.Marshal(updateData)
	updateReq, _ := http.NewRequest("PATCH", updateURL, io.NopCloser(bytes.NewBuffer(updateJSON)))
	updateReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	updateReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Prefer", "return=minimal")
	
	updateResp, _ := httpclient.Client.Do(updateReq)
	defer updateResp.Body.Close()

	responsex.RespondWithJSON(w, http.StatusOK, models.Response{
		Code: http.StatusOK,
		Msg:  "订阅已成功取消，将在当前计费周期结束后停止",
		Data: map[string]interface{}{
			"subscription_id": subscription.SubscriptionID,
			"end_date":        subscription.CurrentPeriodEnd,
		},
	})
}

// UpdateSubscription 更新订阅
func UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NewPlanID string `json:"new_plan_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsex.RespondWithJSON(w, http.StatusBadRequest, models.Response{
			Code: http.StatusBadRequest,
			Msg:  "无效的请求格式",
			Data: map[string]interface{}{},
		})
		return
	}

	userID := auth.GetUserIDFromContext(r)
	
	// 获取当前订阅
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("status", "eq.active")
	queryParams.Add("limit", "1")
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	httpReq, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	httpReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	httpReq.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(httpReq)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	var subscriptions []tables.StripeSubscription
	err = json.Unmarshal(bodyBytes, &subscriptions)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	if len(subscriptions) == 0 {
		responsex.RespondWithJSON(w, http.StatusBadRequest, models.Response{
			Code: http.StatusBadRequest,
			Msg:  "没有找到活跃的订阅",
			Data: map[string]interface{}{},
		})
		return
	}

	// 更新订阅
	subscription := subscriptions[0]
	updatedSub, err := stripeService.UpdateSubscription(subscription.SubscriptionID, req.NewPlanID)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "更新订阅失败",
			Data: map[string]interface{}{},
		})
		return
	}

	// 更新数据库中的订阅信息
	updateURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions?id=eq.%s", config.Cfg.Supabase.SupabaseUrl, subscription.ID)
	updateData := map[string]interface{}{
		"plan_id":     req.NewPlanID,
		"updated_at":  time.Now(),
	}
	
	updateJSON, _ := json.Marshal(updateData)
	updateReq, _ := http.NewRequest("PATCH", updateURL, bytes.NewBuffer(updateJSON))
	updateReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	updateReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Prefer", "return=minimal")
	
	updateResp, _ := httpclient.Client.Do(updateReq)
	defer updateResp.Body.Close()

	// 获取新计划详情
	newPlan, err := getPlanDetails(req.NewPlanID)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	responsex.RespondWithJSON(w, http.StatusOK, models.Response{
		Code: http.StatusOK,
		Msg:  "订阅已成功更新",
		Data: map[string]interface{}{
			"subscription_id": updatedSub.ID,
			"new_plan": PlanInfo{
				ID:             newPlan.PlanID,
				Name:           newPlan.Name,
				Price:          newPlan.Amount / 100,
				Currency:       newPlan.Currency,
				Interval:       newPlan.Interval,
				CharacterLimit: newPlan.CharacterLimit,
				Support:        "24/7",
				Webhook:        true,
			},
		},
	})
}

// GetPaymentMethods 获取用户的支付方式
func GetPaymentMethods(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r)
	
	// 获取用户的Stripe客户ID
	customerID, err := getStripeCustomerID(userID)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	if customerID == "" {
		responsex.RespondWithJSON(w, http.StatusOK, models.Response{
			Code: http.StatusOK,
			Msg:  "成功",
			Data: []PaymentMethodInfo{},
		})
		return
	}

	// 获取支付方式
	paymentMethods, err := stripeService.GetPaymentMethods(customerID)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "获取支付方式失败",
			Data: map[string]interface{}{},
		})
		return
	}

	// 转换为前端所需格式
	var paymentMethodInfos []PaymentMethodInfo
	for _, pm := range paymentMethods {
		if pm.Card != nil {
			paymentMethodInfos = append(paymentMethodInfos, PaymentMethodInfo{
				ID:        pm.ID,
				Type:      string(pm.Type),
				Last4:     pm.Card.Last4,
				Brand:     string(pm.Card.Brand),
				ExpMonth:  int(pm.Card.ExpMonth),
				ExpYear:   int(pm.Card.ExpYear),
				IsDefault: pm.ID == customerID,
			})
		}
	}

	responsex.RespondWithJSON(w, http.StatusOK, models.Response{
		Code: http.StatusOK,
		Msg:  "成功",
		Data: paymentMethodInfos,
	})
}

// GetInvoices 获取用户的发票
func GetInvoices(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserIDFromContext(r)
	
	// 获取用户的Stripe客户ID
	customerID, err := getStripeCustomerID(userID)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "内部服务器错误",
			Data: map[string]interface{}{},
		})
		return
	}

	if customerID == "" {
		responsex.RespondWithJSON(w, http.StatusOK, models.Response{
			Code: http.StatusOK,
			Msg:  "成功",
			Data: []InvoiceInfo{},
		})
		return
	}

	// 获取发票
	invoices, err := stripeService.GetInvoices(customerID)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusInternalServerError, models.Response{
			Code: http.StatusInternalServerError,
			Msg:  "获取发票失败",
			Data: map[string]interface{}{},
		})
		return
	}

	// 转换为前端所需格式
	var invoiceInfos []InvoiceInfo
	for _, inv := range invoices {
		invoiceInfos = append(invoiceInfos, InvoiceInfo{
			ID:         inv.ID,
			Amount:     int(inv.AmountPaid),
			Currency:   string(inv.Currency),
			Status:     string(inv.Status),
			InvoiceURL: inv.InvoicePDF,
			InvoicePDF: inv.InvoicePDF,
			CreatedAt:  time.Unix(inv.Created, 0),
		})
	}

	responsex.RespondWithJSON(w, http.StatusOK, models.Response{
		Code: http.StatusOK,
		Msg:  "成功",
		Data: invoiceInfos,
	})
}

// HandleWebhook 处理Stripe Webhook事件
func HandleWebhook(w http.ResponseWriter, r *http.Request) {
	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusBadRequest, models.Response{
			Code: http.StatusBadRequest,
			Msg:  "读取请求体失败",
			Data: map[string]interface{}{},
		})
		return
	}

	// 验证Webhook签名
	endpointSecret := config.Cfg.Stripe.WebhookSecret
	event, err := stripeService.VerifyWebhookSignature(payload, r.Header.Get("Stripe-Signature"), endpointSecret)
	if err != nil {
		responsex.RespondWithJSON(w, http.StatusBadRequest, models.Response{
			Code: http.StatusBadRequest,
			Msg:  "无效的Webhook签名",
			Data: map[string]interface{}{},
		})
		return
	}

	// 处理不同类型的事件
	switch event.Type {
	case "customer.subscription.created":
		handleSubscriptionCreated(event)
	case "customer.subscription.updated":
		handleSubscriptionUpdated(event)
	case "customer.subscription.deleted":
		handleSubscriptionDeleted(event)
	case "invoice.payment_succeeded":
		handleInvoicePaymentSucceeded(event)
	case "invoice.payment_failed":
		handleInvoicePaymentFailed(event)
	}

	// 记录Webhook事件
	saveWebhookEvent(event)

	w.WriteHeader(http.StatusOK)
}

// 辅助函数

// getOrCreateStripeCustomer 获取或创建Stripe客户
func getOrCreateStripeCustomer(userID string) (string, error) {
	// 先尝试获取现有客户ID
	customerID, err := getStripeCustomerID(userID)
	if err != nil {
		return "", err
	}

	if customerID != "" {
		return customerID, nil
	}

	// 获取用户信息
	baseURL := fmt.Sprintf("%s/rest/v1/users", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("id", "eq."+userID)
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

	var users []tables.User
	err = json.Unmarshal(bodyBytes, &users)
	if err != nil {
		return "", err
	}

	if len(users) == 0 {
		return "", fmt.Errorf("用户不存在")
	}

	user := users[0]

	// 创建Stripe客户
	customer, err := stripeService.CreateCustomer(user.Email, user.ID)
	if err != nil {
		return "", err
	}

	// 保存客户ID到数据库
	saveURL := fmt.Sprintf("%s/rest/v1/stripe_customers", config.Cfg.Supabase.SupabaseUrl)
	saveData := map[string]interface{}{
		"user_id":      userID,
		"customer_id":  customer.ID,
		"email":        user.Email,
		"created_at":   time.Now(),
	}
	
	saveJSON, _ := json.Marshal(saveData)
	saveReq, _ := http.NewRequest("POST", saveURL, bytes.NewBuffer(saveJSON))
	saveReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	saveReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.Header.Set("Prefer", "return=minimal")
	
	saveResp, _ := httpclient.Client.Do(saveReq)
	defer saveResp.Body.Close()

	return customer.ID, nil
}

// getStripeCustomerID 获取用户的Stripe客户ID
func getStripeCustomerID(userID string) (string, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_customers", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "customer_id")
	queryParams.Add("user_id", "eq."+userID)
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

	var customers []struct {
		CustomerID string `json:"customer_id"`
	}
	err = json.Unmarshal(bodyBytes, &customers)
	if err != nil {
		return "", err
	}

	if len(customers) == 0 {
		return "", nil
	}

	return customers[0].CustomerID, nil
}

// getPlanDetails 获取计划详情
func getPlanDetails(planID string) (*tables.StripePlan, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_plans", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("plan_id", "eq."+planID)
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
		return nil, fmt.Errorf("计划不存在")
	}

	return &plans[0], nil
}

// 处理Webhook事件的函数

// handleSubscriptionCreated 处理订阅创建事件
func handleSubscriptionCreated(event stripe.Event) {
	var subscription stripe.Subscription
	err := json.Unmarshal(event.Data.Raw, &subscription)
	if err != nil {
		log.Printf("解析订阅创建事件失败: %v", err)
		return
	}

	// 获取用户ID
	userID, err := getUserIDFromCustomerID(subscription.Customer.ID)
	if err != nil {
		log.Printf("获取用户ID失败: %v", err)
		return
	}

	// 保存订阅信息到数据库
	saveURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions", config.Cfg.Supabase.SupabaseUrl)
	saveData := map[string]interface{}{
		"user_id":             userID,
		"subscription_id":     subscription.ID,
		"customer_id":         subscription.Customer.ID,
		"plan_id":             subscription.Plan.ID,
		"status":              string(subscription.Status),
		"current_period_start": time.Unix(subscription.CurrentPeriodStart, 0),
		"current_period_end":   time.Unix(subscription.CurrentPeriodEnd, 0),
		"cancel_at_period_end": subscription.CancelAtPeriodEnd,
		"created_at":           time.Now(),
	}
	
	saveJSON, _ := json.Marshal(saveData)
	saveReq, _ := http.NewRequest("POST", saveURL, bytes.NewBuffer(saveJSON))
	saveReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	saveReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.Header.Set("Prefer", "return=minimal")
	
	saveResp, _ := httpclient.Client.Do(saveReq)
	defer saveResp.Body.Close()

	// 更新用户配额
	updateUserQuota(userID, subscription.Plan.ID)
}

// handleSubscriptionUpdated 处理订阅更新事件
func handleSubscriptionUpdated(event stripe.Event) {
	var subscription stripe.Subscription
	err := json.Unmarshal(event.Data.Raw, &subscription)
	if err != nil {
		log.Printf("解析订阅更新事件失败: %v", err)
		return
	}

	// 获取用户ID
	userID, err := getUserIDFromCustomerID(subscription.Customer.ID)
	if err != nil {
		log.Printf("获取用户ID失败: %v", err)
		return
	}

	// 更新数据库中的订阅信息
	updateURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions?subscription_id=eq.%s", config.Cfg.Supabase.SupabaseUrl, subscription.ID)
	updateData := map[string]interface{}{
		"plan_id":              subscription.Plan.ID,
		"status":               string(subscription.Status),
		"current_period_start": time.Unix(subscription.CurrentPeriodStart, 0),
		"current_period_end":   time.Unix(subscription.CurrentPeriodEnd, 0),
		"cancel_at_period_end": subscription.CancelAtPeriodEnd,
		"updated_at":           time.Now(),
	}
	
	updateJSON, _ := json.Marshal(updateData)
	updateReq, _ := http.NewRequest("PATCH", updateURL, bytes.NewBuffer(updateJSON))
	updateReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	updateReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Prefer", "return=minimal")
	
	updateResp, _ := httpclient.Client.Do(updateReq)
	defer updateResp.Body.Close()

	// 更新用户配额
	updateUserQuota(userID, subscription.Plan.ID)
}

// handleSubscriptionDeleted 处理订阅删除事件
func handleSubscriptionDeleted(event stripe.Event) {
	var subscription stripe.Subscription
	err := json.Unmarshal(event.Data.Raw, &subscription)
	if err != nil {
		log.Printf("解析订阅删除事件失败: %v", err)
		return
	}

	// 更新数据库中的订阅状态
	updateURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions?subscription_id=eq.%s", config.Cfg.Supabase.SupabaseUrl, subscription.ID)
	updateData := map[string]interface{}{
		"status":     "canceled",
		"updated_at": time.Now(),
	}
	
	updateJSON, _ := json.Marshal(updateData)
	updateReq, _ := http.NewRequest("PATCH", updateURL, bytes.NewBuffer(updateJSON))
	updateReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	updateReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Prefer", "return=minimal")
	
	updateResp, _ := httpclient.Client.Do(updateReq)
	defer updateResp.Body.Close()

	// 获取用户ID
	userID, err := getUserIDFromCustomerID(subscription.Customer.ID)
	if err != nil {
		log.Printf("获取用户ID失败: %v", err)
		return
	}

	// 将用户降级为免费计划
	updateUserQuota(userID, "free_plan")
}

// handleInvoicePaymentSucceeded 处理发票支付成功事件
func handleInvoicePaymentSucceeded(event stripe.Event) {
	var invoice stripe.Invoice
	err := json.Unmarshal(event.Data.Raw, &invoice)
	if err != nil {
		log.Printf("解析发票支付成功事件失败: %v", err)
		return
	}

	// 获取用户ID
	userID, err := getUserIDFromCustomerID(invoice.Customer.ID)
	if err != nil {
		log.Printf("获取用户ID失败: %v", err)
		return
	}

	// 保存发票信息到数据库
	saveURL := fmt.Sprintf("%s/rest/v1/stripe_invoices", config.Cfg.Supabase.SupabaseUrl)
	saveData := map[string]interface{}{
		"user_id":      userID,
		"invoice_id":   invoice.ID,
		"customer_id":  invoice.Customer.ID,
		"amount":       invoice.AmountPaid,
		"currency":     string(invoice.Currency),
		"status":       string(invoice.Status),
		"invoice_url":  invoice.InvoicePDF,
		"invoice_pdf":  invoice.InvoicePDF,
		"created_at":   time.Unix(invoice.Created, 0),
	}
	
	saveJSON, _ := json.Marshal(saveData)
	saveReq, _ := http.NewRequest("POST", saveURL, bytes.NewBuffer(saveJSON))
	saveReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	saveReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.Header.Set("Prefer", "return=minimal")
	
	saveResp, _ := httpclient.Client.Do(saveReq)
	defer saveResp.Body.Close()

	// 发送支付成功通知
	sendPaymentSuccessNotification(userID, invoice.ID, invoice.AmountPaid)
}

// handleInvoicePaymentFailed 处理发票支付失败事件
func handleInvoicePaymentFailed(event stripe.Event) {
	var invoice stripe.Invoice
	err := json.Unmarshal(event.Data.Raw, &invoice)
	if err != nil {
		log.Printf("解析发票支付失败事件失败: %v", err)
		return
	}

	// 获取用户ID
	userID, err := getUserIDFromCustomerID(invoice.Customer.ID)
	if err != nil {
		log.Printf("获取用户ID失败: %v", err)
		return
	}

	// 保存发票信息到数据库
	saveURL := fmt.Sprintf("%s/rest/v1/stripe_invoices", config.Cfg.Supabase.SupabaseUrl)
	saveData := map[string]interface{}{
		"user_id":      userID,
		"invoice_id":   invoice.ID,
		"customer_id":  invoice.Customer.ID,
		"amount":       invoice.AmountDue,
		"currency":     string(invoice.Currency),
		"status":       string(invoice.Status),
		"invoice_url":  invoice.InvoicePDF,
		"invoice_pdf":  invoice.InvoicePDF,
		"created_at":   time.Unix(invoice.Created, 0),
	}
	
	saveJSON, _ := json.Marshal(saveData)
	saveReq, _ := http.NewRequest("POST", saveURL, bytes.NewBuffer(saveJSON))
	saveReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	saveReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.Header.Set("Prefer", "return=minimal")
	
	saveResp, _ := httpclient.Client.Do(saveReq)
	defer saveResp.Body.Close()

	// 发送支付失败通知
	sendPaymentFailedNotification(userID, invoice.ID, invoice.AmountDue)
}

// getUserIDFromCustomerID 根据Stripe客户ID获取用户ID
func getUserIDFromCustomerID(customerID string) (string, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_customers", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "user_id")
	queryParams.Add("customer_id", "eq."+customerID)
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

	var customers []struct {
		UserID string `json:"user_id"`
	}
	err = json.Unmarshal(bodyBytes, &customers)
	if err != nil {
		return "", err
	}

	if len(customers) == 0 {
		return "", fmt.Errorf("客户不存在")
	}

	return customers[0].UserID, nil
}

// updateUserQuota 更新用户配额
func updateUserQuota(userID string, planID string) error {
	// 获取计划详情
	plan, err := getPlanDetails(planID)
	if err != nil {
		return err
	}

	// 更新用户配额
	updateURL := fmt.Sprintf("%s/rest/v1/user_quotas?user_id=eq.%s", config.Cfg.Supabase.SupabaseUrl, userID)
	
	// 先检查用户配额是否存在
	checkReq, _ := http.NewRequest("GET", updateURL, nil)
	checkReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	checkReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	checkReq.Header.Set("Accept", "application/json")
	
	checkResp, _ := httpclient.Client.Do(checkReq)
	defer checkResp.Body.Close()
	
	checkBytes, _ := io.ReadAll(checkResp.Body)
	var quotas []struct {
		ID string `json:"id"`
	}
	json.Unmarshal(checkBytes, &quotas)
	
	var method string
	var reqURL string
	
	if len(quotas) == 0 {
		// 创建新配额
		method = "POST"
		reqURL = fmt.Sprintf("%s/rest/v1/user_quotas", config.Cfg.Supabase.SupabaseUrl)
	} else {
		// 更新现有配额
		method = "PATCH"
		reqURL = updateURL
	}
	
	updateData := map[string]interface{}{
		"user_id":          userID,
		"plan_id":          planID,
		"character_limit":  plan.CharacterLimit,
		"updated_at":       time.Now(),
	}
	
	updateJSON, _ := json.Marshal(updateData)
	updateReq, _ := http.NewRequest(method, reqURL, bytes.NewBuffer(updateJSON))
	updateReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	updateReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Prefer", "return=minimal")
	
	updateResp, _ := httpclient.Client.Do(updateReq)
	defer updateResp.Body.Close()

	return nil
}

// saveWebhookEvent 保存Webhook事件到数据库
func saveWebhookEvent(event stripe.Event) {
	saveURL := fmt.Sprintf("%s/rest/v1/stripe_webhook_events", config.Cfg.Supabase.SupabaseUrl)
	saveData := map[string]interface{}{
		"event_id":   event.ID,
		"event_type": event.Type,
		"data":       string(event.Data.Raw),
		"created_at": time.Now(),
	}
	
	saveJSON, _ := json.Marshal(saveData)
	saveReq, _ := http.NewRequest("POST", saveURL, bytes.NewBuffer(saveJSON))
	saveReq.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	saveReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.Header.Set("Prefer", "return=minimal")
	
	saveResp, _ := httpclient.Client.Do(saveReq)
	defer saveResp.Body.Close()
}

// sendPaymentSuccessNotification 发送支付成功通知
func sendPaymentSuccessNotification(userID string, invoiceID string, amount int64) {
	// 获取用户信息
	baseURL := fmt.Sprintf("%s/rest/v1/users", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "email")
	queryParams.Add("id", "eq."+userID)
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, _ := httpclient.Client.Do(req)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var users []struct {
		Email string `json:"email"`
	}
	json.Unmarshal(bodyBytes, &users)

	if len(users) == 0 {
		return
	}

	// TODO: 实现发送邮件的逻辑
	// 这里可以调用邮件服务发送支付成功通知
	log.Printf("发送支付成功通知到 %s，发票ID: %s，金额: %d", users[0].Email, invoiceID, amount)
}

// sendPaymentFailedNotification 发送支付失败通知
func sendPaymentFailedNotification(userID string, invoiceID string, amount int64) {
	// 获取用户信息
	baseURL := fmt.Sprintf("%s/rest/v1/users", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "email")
	queryParams.Add("id", "eq."+userID)
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, _ := httpclient.Client.Do(req)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var users []struct {
		Email string `json:"email"`
	}
	json.Unmarshal(bodyBytes, &users)

	if len(users) == 0 {
		return
	}

	// TODO: 实现发送邮件的逻辑
	// 这里可以调用邮件服务发送支付失败通知
	log.Printf("发送支付失败通知到 %s，发票ID: %s，金额: %d", users[0].Email, invoiceID, amount)
}
