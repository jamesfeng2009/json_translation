package stripe

import (
	"encoding/json"
	"errors"
	"fmt"
	"json_trans_api/config"
	"json_trans_api/models/tables"
	"log"
	"time"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/checkout/session"
	"github.com/stripe/stripe-go/v72/customer"
	"github.com/stripe/stripe-go/v72/invoice"
	"github.com/stripe/stripe-go/v72/paymentmethod"
	"github.com/stripe/stripe-go/v72/plan"
	"github.com/stripe/stripe-go/v72/sub"
	"github.com/stripe/stripe-go/v72/webhook"
)

// 初始化Stripe
func Init() {
	stripe.Key = config.Cfg.Stripe.SecretKey
}

// CreateCustomer 创建Stripe客户
func CreateCustomer(email, userID string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"user_id": userID,
		},
	}

	return customer.New(params)
}

// GetCustomer 获取Stripe客户
func GetCustomer(customerID string) (*stripe.Customer, error) {
	return customer.Get(customerID, nil)
}

// CreateCheckoutSession 创建结账会话
func CreateCheckoutSession(customerID, planID, successURL, cancelURL string) (*stripe.CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(planID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Customer:   stripe.String(customerID),
	}

	return session.New(params)
}

// GetSubscription 获取订阅信息
func GetSubscription(subscriptionID string) (*stripe.Subscription, error) {
	return sub.Get(subscriptionID, nil)
}

// CancelSubscription 取消订阅
func CancelSubscription(subscriptionID string, cancelAtPeriodEnd bool) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(cancelAtPeriodEnd),
	}
	return sub.Update(subscriptionID, params)
}

// UpdateSubscription 更新订阅
func UpdateSubscription(subscriptionID, newPlanID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{
				ID:    nil, // 将在更新时自动获取
				Price: stripe.String(newPlanID),
			},
		},
	}
	return sub.Update(subscriptionID, params)
}

// GetInvoice 获取发票
func GetInvoice(invoiceID string) (*stripe.Invoice, error) {
	return invoice.Get(invoiceID, nil)
}

// ListCustomerInvoices 列出客户的所有发票
func ListCustomerInvoices(customerID string) ([]*stripe.Invoice, error) {
	params := &stripe.InvoiceListParams{
		Customer: stripe.String(customerID),
	}
	
	var invoices []*stripe.Invoice
	i := invoice.List(params)
	for i.Next() {
		invoices = append(invoices, i.Invoice())
	}
	
	return invoices, i.Err()
}

// AddPaymentMethod 添加支付方式
func AddPaymentMethod(customerID, paymentMethodID string) (*stripe.PaymentMethod, error) {
	params := &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(customerID),
	}
	return paymentmethod.Attach(paymentMethodID, params)
}

// SetDefaultPaymentMethod 设置默认支付方式
func SetDefaultPaymentMethod(customerID, paymentMethodID string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(paymentMethodID),
		},
	}
	return customer.Update(customerID, params)
}

// VerifyWebhookSignature 验证Webhook签名
func VerifyWebhookSignature(payload []byte, signature, secret string) (stripe.Event, error) {
	return webhook.ConstructEvent(payload, signature, secret)
}

// GetPlanDetails 获取计划详情
func GetPlanDetails(planID string) (*stripe.Plan, error) {
	return plan.Get(planID, nil)
}

// ConvertToSubscriptionModel 将Stripe订阅转换为本地模型
func ConvertToSubscriptionModel(sub *stripe.Subscription, userID string) tables.StripeSubscription {
	return tables.StripeSubscription{
		UserID:            userID,
		CustomerID:        sub.Customer.ID,
		SubscriptionID:    sub.ID,
		Status:            string(sub.Status),
		PlanID:            sub.Items.Data[0].Price.ID,
		CurrentPeriodStart: time.Unix(sub.CurrentPeriodStart, 0).Format(time.RFC3339),
		CurrentPeriodEnd:   time.Unix(sub.CurrentPeriodEnd, 0).Format(time.RFC3339),
		CancelAtPeriodEnd:  sub.CancelAtPeriodEnd,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

// ConvertToPlanModel 将Stripe计划转换为本地模型
func ConvertToPlanModel(p *stripe.Plan) tables.StripePlan {
	characterLimit := 10000 // 默认字符限制
	
	// 从元数据中获取字符限制
	if p.Metadata != nil {
		if limit, ok := p.Metadata["character_limit"]; ok {
			fmt.Sscanf(limit, "%d", &characterLimit)
		}
	}
	
	return tables.StripePlan{
		PlanID:         p.ID,
		Name:           p.Nickname,
		Description:    "",
		Amount:         int(p.Amount),
		Currency:       string(p.Currency),
		Interval:       string(p.Interval),
		CharacterLimit: characterLimit,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}