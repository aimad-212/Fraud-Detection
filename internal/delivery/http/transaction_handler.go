package http

import (
	"net/http"
	"strconv"

	"fraud-detection/internal/domain"
	"fraud-detection/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TransactionHandler handles HTTP requests for transaction operations
type TransactionHandler struct {
	transactionUseCase usecase.TransactionUseCase
	logger             *zap.Logger
}

// NewTransactionHandler creates a new transaction handler
func NewTransactionHandler(transactionUseCase usecase.TransactionUseCase, logger *zap.Logger) *TransactionHandler {
	return &TransactionHandler{
		transactionUseCase: transactionUseCase,
		logger:             logger,
	}
}

// CreateTransactionRequest represents the request payload for creating a transaction
type CreateTransactionRequest struct {
	UserID    string  `json:"user_id" binding:"required,uuid"`
	Amount    float64 `json:"amount" binding:"required,gt=0"`
	Currency  string  `json:"currency" binding:"omitempty,len=3"`
	Merchant  string  `json:"merchant" binding:"omitempty,max=255"`
	Country   string  `json:"country" binding:"omitempty,len=2"`
	City      string  `json:"city" binding:"omitempty,max=100"`
	IPAddress string  `json:"ip_address,omitempty"`
}

// CreateTransaction handles POST /transactions
func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	var req CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Parse user ID
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		h.logger.Error("Invalid user ID format", zap.Error(err), zap.String("user_id", req.UserID))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	// Set default currency if not provided
	if req.Currency == "" {
		req.Currency = "USD"
	}

	// Extract IP address from request if not provided
	if req.IPAddress == "" {
		req.IPAddress = c.ClientIP()
	}

	// Create transaction
	transaction, err := h.transactionUseCase.CreateTransaction(userID, req.Amount, req.Currency, req.Merchant)
	if err != nil {
		h.logger.Error("Failed to create transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create transaction",
		})
		return
	}

	// Update additional fields if provided
	if req.Country != "" {
		transaction.Country = req.Country
	}
	if req.City != "" {
		transaction.City = req.City
	}
	if req.IPAddress != "" {
		transaction.IPAddress = req.IPAddress
	}

	// Process the transaction (apply fraud detection)
	err = h.transactionUseCase.ProcessTransaction(transaction)
	if err != nil {
		h.logger.Error("Failed to process transaction", zap.Error(err), zap.String("transaction_id", transaction.ID.String()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process transaction",
		})
		return
	}

	h.logger.Info("Transaction created and processed",
		zap.String("transaction_id", transaction.ID.String()),
		zap.String("user_id", userID.String()),
		zap.Float64("amount", req.Amount),
		zap.String("status", string(transaction.Status)))

	c.JSON(http.StatusCreated, gin.H{
		"transaction": transaction,
		"message":     "Transaction created and processed successfully",
	})
}

// GetTransaction handles GET /transactions/:id
func (h *TransactionHandler) GetTransaction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Error("Invalid transaction ID format", zap.Error(err), zap.String("transaction_id", idStr))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid transaction ID format",
		})
		return
	}

	transaction, err := h.transactionUseCase.GetTransaction(id)
	if err != nil {
		h.logger.Error("Failed to get transaction", zap.Error(err), zap.String("transaction_id", id.String()))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Transaction not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transaction": transaction,
	})
}

// GetUserTransactions handles GET /users/:userId/transactions
func (h *TransactionHandler) GetUserTransactions(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error("Invalid user ID format", zap.Error(err), zap.String("user_id", userIDStr))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	transactions, err := h.transactionUseCase.GetUserTransactions(userID)
	if err != nil {
		h.logger.Error("Failed to get user transactions", zap.Error(err), zap.String("user_id", userID.String()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user transactions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": transactions,
		"count":        len(transactions),
	})
}

// UpdateTransactionStatus handles PUT /transactions/:id/status
func (h *TransactionHandler) UpdateTransactionStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Error("Invalid transaction ID format", zap.Error(err), zap.String("transaction_id", idStr))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid transaction ID format",
		})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required,oneof=pending approved rejected flagged"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	status := domain.TransactionStatus(req.Status)
	err = h.transactionUseCase.UpdateTransactionStatus(id, status)
	if err != nil {
		h.logger.Error("Failed to update transaction status", zap.Error(err), zap.String("transaction_id", id.String()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update transaction status",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Transaction status updated successfully",
	})
}

// ListTransactions handles GET /transactions
func (h *TransactionHandler) ListTransactions(c *gin.Context) {
	// Parse query parameters with defaults
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50 // Default limit
	}
	if limit > 100 {
		limit = 100 // Max limit to prevent memory issues
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	transactions, err := h.transactionUseCase.ListTransactions(limit, offset)
	if err != nil {
		h.logger.Error("Failed to list transactions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list transactions",
		})
		return
	}

	// Return paginated results with metadata
	c.JSON(http.StatusOK, gin.H{
		"transactions": transactions,
		"count":        len(transactions),
		"limit":        limit,
		"offset":       offset,
		"pagination": gin.H{
			"limit":       limit,
			"offset":      offset,
			"has_more":    len(transactions) == limit, // Simple indicator
			"total_count": len(transactions),          // In production, add total count query
		},
	})
}

// DeleteTransaction handles DELETE /transactions/:id
func (h *TransactionHandler) DeleteTransaction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Error("Invalid transaction ID format", zap.Error(err), zap.String("transaction_id", idStr))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid transaction ID format",
		})
		return
	}

	err = h.transactionUseCase.DeleteTransaction(id)
	if err != nil {
		h.logger.Error("Failed to delete transaction", zap.Error(err), zap.String("transaction_id", id.String()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete transaction",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Transaction deleted successfully",
	})
}
