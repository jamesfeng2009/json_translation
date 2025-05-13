package stripe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"json_trans_api/config"
	"json_trans_api/models/models"
	"json_trans_api/pkg/httpclient"
	responsex "json_trans_api/pkg/response"
	stripeService "json_trans_api/pkg/stripe"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v72"
)

// 用于保护并发访问的互斥锁
var (
	subscriptionMutex sync.Mutex
	usageMutex        sync.Mutex
)

// HandleWebhook 处理Stripe Webhook事件
func HandleWebhook(w http.ResponseWriter, r *http.Request) {
	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("读取请求体失败: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 验证Webhook签名
	endpointSecret := config.Cfg.Stripe.WebhookSecret
	event, err := stripeService.VerifyWebhookSignature(payload, r.Header.Get("Stripe-Signature"), endpointSecret)
	if err != nil {
		log.Printf("无效的Webhook签名: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 处理不同类型的事件
	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			log.Printf("解析会话数据失败: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		handleCheckoutSessionCompleted(&session)

	case "customer.subscription.created", "customer.subscription.updated":
		var subscription stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			log.Printf("解析订阅数据失败: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		handleSubscriptionUpdated(&subscription)

	case "customer.subscription.deleted":
		var subscription stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			log.Printf("解析订阅数据失败: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		handleSubscriptionDeleted(&subscription)

	case "invoice.paid":
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			log.Printf("解析发票数据失败: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		handleInvoicePaid(&invoice)

	case "invoice.payment_failed":
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			log.Printf("解析发票数据失败: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		handleInvoicePaymentFailed(&invoice)
	}

	// 记录Webhook事件
	if err := saveWebhookEvent(event); err != nil {
		log.Printf("保存Webhook事件失败: %v", err)
	}

	w.WriteHeader(http.StatusOK)
}

// handleCheckoutSessionCompleted 处理结账会话完成事件
func handleCheckoutSessionCompleted(session *stripe.CheckoutSession) {
	// 获取客户ID和订阅ID
	customerID := session.Customer.ID
	subscriptionID := session.Subscription.ID

	// 获取用户ID
	userID, err := getUserIDByStripeCustomerID(customerID)
	if err != nil {
		log.Printf("获取用户ID失败: %v", err)
		return
	}

	// 获取订阅详情
	subscription, err := stripeService.GetSubscription(subscriptionID)
	if err != nil {
		log.Printf("获取订阅详情失败: %v", err)
		return
	}

	// 获取计划ID
	planID := subscription.Items.Data[0].Price.ID

	// 获取计划详情
	plan, err := getPlanDetails(planID)
	if err != nil {
		log.Printf("获取计划详情失败: %v", err)
		return
	}

	// 保存订阅信息到数据库
	if err := saveSubscription(userID, customerID, subscriptionID, planID, subscription.Status, subscription.CurrentPeriodEnd, plan.CharacterLimit); err != nil {
		log.Printf("保存订阅信息失败: %v", err)
	}
}

// handleSubscriptionUpdated 处理订阅更新事件
func handleSubscriptionUpdated(subscription *stripe.Subscription) {
	// 获取客户ID
	customerID := subscription.Customer.ID

	// 获取用户ID
	userID, err := getUserIDByStripeCustomerID(customerID)
	if err != nil {
		log.Printf("获取用户ID失败: %v", err)
		return
	}

	// 获取计划ID
	planID := subscription.Items.Data[0].Price.ID

	// 获取计划详情
	plan, err := getPlanDetails(planID)
	if err != nil {
		log.Printf("获取计划详情失败: %v", err)
		return
	}

	// 更新数据库中的订阅信息
	if err := updateSubscriptionInDB(userID, subscription.ID, planID, string(subscription.Status), time.Unix(subscription.CurrentPeriodEnd, 0), plan.CharacterLimit); err != nil {
		log.Printf("更新订阅信息失败: %v", err)
	}
}

// handleSubscriptionDeleted 处理订阅删除事件
func handleSubscriptionDeleted(subscription *stripe.Subscription) {
	// 获取客户ID
	customerID := subscription.Customer.ID

	// 获取用户ID
	userID, err := getUserIDByStripeCustomerID(customerID)
	if err != nil {
		log.Printf("获取用户ID失败: %v", err)
		return
	}

	// 更新数据库中的订阅状态
	if err := updateSubscriptionStatus(userID, subscription.ID, string(subscription.Status)); err != nil {
		log.Printf("更新订阅状态失败: %v", err)
	}
}

// handleInvoicePaid 处理发票支付成功事件
func handleInvoicePaid(invoice *stripe.Invoice) {
	// 获取客户ID
	customerID := invoice.Customer.ID

	// 获取用户ID
	userID, err := getUserIDByStripeCustomerID(customerID)
	if err != nil {
		log.Printf("获取用户ID失败: %v", err)
		return
	}

	// 保存发票信息
	if err := saveInvoice(userID, invoice.ID, invoice.AmountPaid, string(invoice.Currency), string(invoice.Status), invoice.HostedInvoiceURL, invoice.InvoicePDF); err != nil {
		log.Printf("保存发票信息失败: %v", err)
	}

	// 如果是订阅发票，更新订阅状态
	if invoice.Subscription != nil {
		if err := updateSubscriptionStatus(userID, invoice.Subscription.ID, "active"); err != nil {
			log.Printf("更新订阅状态失败: %v", err)
		}
	}
}

// handleInvoicePaymentFailed 处理发票支付失败事件
func handleInvoicePaymentFailed(invoice *stripe.Invoice) {
	// 获取客户ID
	customerID := invoice.Customer.ID

	// 获取用户ID
	userID, err := getUserIDByStripeCustomerID(customerID)
	if err != nil {
		log.Printf("获取用户ID失败: %v", err)
		return
	}

	// 保存发票信息
	if err := saveInvoice(userID, invoice.ID, invoice.AmountPaid, string(invoice.Currency), string(invoice.Status), invoice.HostedInvoiceURL, invoice.InvoicePDF); err != nil {
		log.Printf("保存发票信息失败: %v", err)
	}

	// 如果是订阅发票，更新订阅状态
	if invoice.Subscription != nil {
		if err := updateSubscriptionStatus(userID, invoice.Subscription.ID, "past_due"); err != nil {
			log.Printf("更新订阅状态失败: %v", err)
		}
	}

	// 发送支付失败通知
	if err := sendPaymentFailedNotification(userID, invoice.ID); err != nil {
		log.Printf("发送支付失败通知失败: %v", err)
	}
}

// saveWebhookEvent 保存Webhook事件到数据库
func saveWebhookEvent(event *stripe.Event) error {
	eventData := map[string]interface{}{
		"event_id":   event.ID,
		"event_type": event.Type,
		"data":       string(event.Data.Raw),
		"created_at": time.Unix(event.Created, 0),
	}
	
	return supabasePost("stripe_webhook_events", eventData)
}

// 辅助函数

// getUserIDByStripeCustomerID 通过Stripe客户ID获取用户ID
func getUserIDByStripeCustomerID(customerID string) (string, error) {
	queryParams := url.Values{}
	queryParams.Add("select", "user_id")
	queryParams.Add("customer_id", "eq."+customerID)
	queryParams.Add("limit", "1")
	
	var customers []struct {
		UserID string `json:"user_id"`
	}
	
	err := supabaseGet("stripe_customers", queryParams, &customers)
	if err != nil {
		return "", err
	}

	if len(customers) == 0 {
		return "", fmt.Errorf("未找到客户")
	}

	return customers[0].UserID, nil
}

// saveSubscription 保存订阅信息到数据库
func saveSubscription(userID, customerID, subscriptionID, planID, status string, periodEnd int64, characterLimit int) error {
	subscriptionMutex.Lock()
	defer subscriptionMutex.Unlock()
	
	subData := map[string]interface{}{
		"user_id":            userID,
		"customer_id":        customerID,
		"subscription_id":    subscriptionID,
		"plan_id":            planID,
		"status":             status,
		"current_period_end": time.Unix(periodEnd, 0),
		"character_limit":    characterLimit,
		"created_at":         time.Now(),
		"updated_at":         time.Now(),
	}
	
	return supabasePost("stripe_subscriptions", subData)
}

// updateSubscriptionInDB 更新数据库中的订阅信息
func updateSubscriptionInDB(userID, subscriptionID, planID, status string, periodEnd time.Time, characterLimit int) error {
	subscriptionMutex.Lock()
	defer subscriptionMutex.Unlock()
	
	queryParams := url.Values{}
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("subscription_id", "eq."+subscriptionID)
	
	updateData := map[string]interface{}{
		"plan_id":            planID,
		"status":             status,
		"current_period_end": periodEnd,
		"character_limit":    characterLimit,
		"updated_at":         time.Now(),
	}
	
	return supabasePatch("stripe_subscriptions", queryParams, updateData)
}

// updateSubscriptionStatus 更新订阅状态
func updateSubscriptionStatus(userID, subscriptionID, status string) error {
	subscriptionMutex.Lock()
	defer subscriptionMutex.Unlock()
	
	queryParams := url.Values{}
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("subscription_id", "eq."+subscriptionID)
	
	updateData := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	
	return supabasePatch("stripe_subscriptions", queryParams, updateData)
}

// saveInvoice 保存发票信息
func saveInvoice(userID, invoiceID string, amount int64, currency, status, invoiceURL, invoicePDF string) error {
	invoiceData := map[string]interface{}{
		"user_id":     userID,
		"invoice_id":  invoiceID,
		"amount":      amount,
		"currency":    currency,
		"status":      status,
		"invoice_url": invoiceURL,
		"invoice_pdf": invoicePDF,
		"created_at":  time.Now(),
	}
	
	return supabasePost("stripe_invoices", invoiceData)
}

// sendPaymentFailedNotification 发送支付失败通知
func sendPaymentFailedNotification(userID, invoiceID string) error {
	log.Printf("向用户 %s 发送发票 %s 支付失败通知", userID, invoiceID)
	
	// 获取用户邮箱
	user, err := getUserInfo(userID)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %v", err)
	}
	
	// 获取发票详情
	invoice, err := stripeService.GetInvoice(invoiceID)
	if err != nil {
		return fmt.Errorf("获取发票详情失败: %v", err)
	}
	
	// 构建通知内容
	notificationData := map[string]interface{}{
		"user_id":     userID,
		"email":       user.Email,
		"subject":     "支付失败通知",
		"message":     fmt.Sprintf("您的发票 %s 支付失败，金额: %d %s，请更新您的支付方式。", invoiceID, invoice.AmountDue/100, string(invoice.Currency)),
		"invoice_url": invoice.HostedInvoiceURL,
		"created_at":  time.Now(),
	}
	
	// 保存通知到数据库
	if err := supabasePost("notifications", notificationData); err != nil {
		return fmt.Errorf("保存通知失败: %v", err)
	}
	
	// TODO: 实现邮件发送逻辑
	// 可以使用第三方邮件服务如 SendGrid, Mailgun 等
	
	return nil
}

// getPlanDetails 获取计划详情
func getPlanDetails(planID string) (*tables.StripePlan, error) {
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("plan_id", "eq."+planID)
	queryParams.Add("limit", "1")
	
	var plans []tables.StripePlan
	
	err := supabaseGet("stripe_plans", queryParams, &plans)
	if err != nil {
		return nil, err
	}

	if len(plans) == 0 {
		return nil, fmt.Errorf("未找到计划")
	}

	return &plans[0], nil
}

// 添加配额检查函数
func checkUserQuota(userID string, characterCount int) (bool, error) {
	usageMutex.Lock()
	defer usageMutex.Unlock()
	
	// 获取用户当前订阅
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("status", "eq.active")
	queryParams.Add("limit", "1")
	
	var subscriptions []tables.StripeSubscription
	
	err := supabaseGet("stripe_subscriptions", queryParams, &subscriptions)
	if err != nil {
		return false, err
	}

	// 如果没有活跃订阅，使用免费配额
	if len(subscriptions) == 0 {
		// 获取本月已使用的字符数
		usedChars, err := getMonthlyUsage(userID)
		if err != nil {
			return false, err
		}
		// 使用配置文件中的免费配额
		return usedChars+characterCount <= config.Cfg.Quota.FreeCharacterLimit, nil
	}

	// 有活跃订阅，检查订阅配额
	subscription := subscriptions[0]
	usedChars, err := getMonthlyUsage(userID)
	if err != nil {
		return false, err
	}

	return usedChars+characterCount <= subscription.CharacterLimit, nil
}

// getMonthlyUsage 获取用户本月已使用的字符数
func getMonthlyUsage(userID string) (int, error) {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	
	queryParams := url.Values{}
	queryParams.Add("select", "character_count")
	queryParams.Add("user_id", "eq."+userID)
	queryParams.Add("created_at", "gte."+startOfMonth.Format(time.RFC3339))
	
	var usageRecords []struct {
		CharacterCount int `json:"character_count"`
	}
	
	err := supabaseGet("usage_records", queryParams, &usageRecords)
	if err != nil {
		return 0, err
	}

	totalUsage := 0
	for _, record := range usageRecords {
		totalUsage += record.CharacterCount
	}

	return totalUsage, nil
}

// 添加一个公共函数，用于在API请求前检查用户配额
func CheckAndRecordUsage(userID string, characterCount int, requestType string) (bool, error) {
	usageMutex.Lock()
	defer usageMutex.Unlock()
	
	// 检查用户配额
	hasQuota, err := checkUserQuota(userID, characterCount)
	if err != nil {
		return false, err
	}

	if !hasQuota {
		return false, nil
	}

	// 记录使用量
	usageData := map[string]interface{}{
		"user_id":         userID,
		"character_count": characterCount,
		"request_type":    requestType,
		"created_at":      time.Now(),
	}
	
	if err := supabasePost("usage_records", usageData); err != nil {
		return true, err
	}
	
	return true, nil
}

// getUserInfo 获取用户信息
func getUserInfo(userID string) (*tables.User, error) {
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("id", "eq."+userID)
	queryParams.Add("limit", "1")
	
	var users []tables.User
	
	err := supabaseGet("users", queryParams, &users)
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("未找到用户")
	}

	return &users[0], nil
}

// 通用的Supabase HTTP请求函数

// supabaseGet 执行Supabase GET请求
func supabaseGet(table string, queryParams url.Values, result interface{}) error {
	baseURL := fmt.Sprintf("%s/rest/v1/%s", config.Cfg.Supabase.SupabaseUrl, table)
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Supabase GET请求失败: %d, %s", resp.StatusCode, string(bodyBytes))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// supabasePost 执行Supabase POST请求
func supabasePost(table string, data map[string]interface{}) error {
	baseURL := fmt.Sprintf("%s/rest/v1/%s", config.Cfg.Supabase.SupabaseUrl, table)
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequest("POST", baseURL, bytes.NewBuffer(jsonData))
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
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Supabase POST请求失败: %d, %s", resp.StatusCode, string(bodyBytes))
	}
	
	return nil
}

// supabasePatch 执行Supabase PATCH请求
func supabasePatch(table string, queryParams url.Values, data map[string]interface{}) error {
	baseURL := fmt.Sprintf("%s/rest/v1/%s", config.Cfg.Supabase.SupabaseUrl, table)
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequest("PATCH", fullURL, bytes.NewBuffer(jsonData))
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
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Supabase PATCH请求失败: %d, %s", resp.StatusCode, string(bodyBytes))
	}
	
	return nil
}