package domain

import (
	"time"

	"github.com/google/uuid"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	StatusPending  TransactionStatus = "pending"
	StatusApproved TransactionStatus = "approved"
	StatusRejected TransactionStatus = "rejected"
	StatusFlagged  TransactionStatus = "flagged"
)

// Transaction represents a financial transaction
type Transaction struct {
	ID          uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID      uuid.UUID         `json:"user_id" gorm:"type:uuid;not null"`
	Amount      float64           `json:"amount" gorm:"type:decimal(15,2);not null"`
	Currency    string            `json:"currency" gorm:"type:varchar(3);not null;default:'USD'"`
	Merchant    string            `json:"merchant" gorm:"type:varchar(255)"`
	Description string            `json:"description" gorm:"type:text"`
	Country     string            `json:"country" gorm:"type:varchar(2)"`
	City        string            `json:"city" gorm:"type:varchar(100)"`
	IPAddress   string            `json:"ip_address" gorm:"type:inet"`
	UserAgent   string            `json:"user_agent" gorm:"type:text"`
	DeviceID    string            `json:"device_id" gorm:"type:varchar(255)"`
	Status      TransactionStatus `json:"status" gorm:"type:varchar(20);not null;default:'pending'"`
	RiskScore   float64           `json:"risk_score" gorm:"type:decimal(5,4);default:0"`
	CreatedAt   time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
}

// NewTransaction creates a new transaction with default values
func NewTransaction(userID uuid.UUID, amount float64, currency string) *Transaction {
	return &Transaction{
		ID:        uuid.New(),
		UserID:    userID,
		Amount:    amount,
		Currency:  currency,
		Status:    StatusPending,
		RiskScore: 0.0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// IsHighRisk returns true if the transaction has a high risk score
func (t *Transaction) IsHighRisk(threshold float64) bool {
	return t.RiskScore >= threshold
}

// IsApproved returns true if the transaction is approved
func (t *Transaction) IsApproved() bool {
	return t.Status == StatusApproved
}

// IsRejected returns true if the transaction is rejected
func (t *Transaction) IsRejected() bool {
	return t.Status == StatusRejected
}

// IsFlagged returns true if the transaction is flagged for review
func (t *Transaction) IsFlagged() bool {
	return t.Status == StatusFlagged
}

// UpdateStatus updates the transaction status and updated timestamp
func (t *Transaction) UpdateStatus(status TransactionStatus) {
	t.Status = status
	t.UpdatedAt = time.Now()
}

// UpdateRiskScore updates the risk score
func (t *Transaction) UpdateRiskScore(score float64) {
	t.RiskScore = score
	t.UpdatedAt = time.Now()
}
