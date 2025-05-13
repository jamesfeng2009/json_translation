package tables

import "time"

// StripeCustomer 表示Stripe客户信息
type StripeCustomer struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	CustomerID   string    `json:"customer_id"` // Stripe客户ID
	Email        string    `json:"email"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// StripeSubscription 表示订阅信息
type StripeSubscription struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	CustomerID        string    `json:"customer_id"`
	SubscriptionID    string    `json:"subscription_id"` // Stripe订阅ID
	Status            string    `json:"status"`          // active, canceled, past_due等
	PlanID            string    `json:"plan_id"`
	CurrentPeriodStart string    `json:"current_period_start"`
	CurrentPeriodEnd   string    `json:"current_period_end"`
	CancelAtPeriodEnd  bool      `json:"cancel_at_period_end"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// StripePlan 表示订阅计划
type StripePlan struct {
	ID              string                 `json:"id"`
	PlanID          string                 `json:"plan_id"` // Stripe计划ID
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Amount          int                    `json:"amount"`
	Currency        string                 `json:"currency"`
	Interval        string                 `json:"interval"` // month, year等
	CharacterLimit  int                    `json:"character_limit"`
	Features        map[string]interface{} `json:"features"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// StripeInvoice 表示发票信息
type StripeInvoice struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	CustomerID    string    `json:"customer_id"`
	InvoiceID     string    `json:"invoice_id"` // Stripe发票ID
	SubscriptionID string    `json:"subscription_id"`
	Amount        int       `json:"amount"`
	Currency      string    `json:"currency"`
	Status        string    `json:"status"` // paid, open, void等
	InvoiceURL    string    `json:"invoice_url"`
	InvoicePDF    string    `json:"invoice_pdf"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// StripePaymentMethod 表示支付方式
type StripePaymentMethod struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	CustomerID      string    `json:"customer_id"`
	PaymentMethodID string    `json:"payment_method_id"` // Stripe支付方式ID
	Type            string    `json:"type"`              // card, bank等
	Last4           string    `json:"last4"`
	Brand           string    `json:"brand"`
	ExpMonth        int       `json:"exp_month"`
	ExpYear         int       `json:"exp_year"`
	IsDefault       bool      `json:"is_default"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}