package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"

	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
	"umrahservice-api/internal/support"
)

// Balance mirrors WalletController::balance.
func (h *Handler) Balance(c *gin.Context) {
	userID := h.principal(c).User.ID

	var totals struct {
		TotalIn  *float64 `gorm:"column:total_in"`
		TotalOut *float64 `gorm:"column:total_out"`
		Balance  *float64 `gorm:"column:balance"`
	}

	h.DB.Raw(`
		SELECT
			SUM(CASE
				WHEN type = 'd' OR (type = 't' AND to_user_id = ?) THEN amount_c
				ELSE 0
			END) AS total_in,
			SUM(CASE
				WHEN type = 'c' OR (type = 't' AND user_id = ?) THEN amount_c
				ELSE 0
			END) AS total_out,
			SUM(CASE
				WHEN type = 'd' THEN amount_c
				WHEN type = 'c' THEN -amount_c
				WHEN type = 't' AND to_user_id = ? THEN amount_c
				WHEN type = 't' AND user_id = ? THEN -amount_c
				ELSE 0
			END) AS balance
		FROM user_cashes
		WHERE user_id = ? OR to_user_id = ?
	`, userID, userID, userID, userID, userID, userID).Scan(&totals)

	c.JSON(http.StatusOK, gin.H{
		"balance":   deref(totals.Balance),
		"total_in":  deref(totals.TotalIn),
		"total_out": deref(totals.TotalOut),
		"currency":  "SAR",
	})
}

// Transactions mirrors WalletController::transactions.
func (h *Handler) Transactions(c *gin.Context) {
	userID := h.principal(c).User.ID

	var txns []models.UserCash
	h.DB.
		Preload("Group.Customer").
		Preload("Category.Parent").
		Where("user_id = ? OR to_user_id = ?", userID, userID).
		Order("cashed_at DESC").
		Limit(100).
		Find(&txns)

	data := make([]gin.H, 0, len(txns))
	for i := range txns {
		data = append(data, h.transformTransaction(&txns[i], userID))
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// Recipients mirrors WalletController::recipients.
func (h *Handler) Recipients(c *gin.Context) {
	var users []gin.H
	rows := []models.User{}
	h.DB.Select("id", "name", "phone").
		Where("last_login_at IS NOT NULL").
		Order("name").
		Find(&rows)
	for _, u := range rows {
		users = append(users, gin.H{"id": u.ID, "name": u.Name, "phone": u.Phone})
	}
	if users == nil {
		users = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}

// Categories mirrors WalletController::categories.
func (h *Handler) Categories(c *gin.Context) {
	p := h.principal(c)
	typ := c.Query("type")
	if typ == "" {
		typ, _ = c.GetPostForm("type")
	}
	if typ != "income" && typ != "expense" && typ != "transfer" {
		validationError(c, map[string][]string{"type": {"The selected type is invalid."}})
		return
	}

	typeMap := map[string]string{
		"income":   enums.CashCategoryIncome,
		"expense":  enums.CashCategoryExpense,
		"transfer": enums.CashCategoryTransfer,
	}
	childType := typeMap[typ]

	q := h.DB.Model(&models.CashCategory{}).
		Select("id", "parent_id", "`group`", "type", "name")

	if p.HasExactRoles(enums.RoleMutawif) {
		q = q.Where("`group` = ?", enums.ExpenseGroupMutawif)
	}
	if p.HasExactRoles(enums.RoleCheckInOutTeam) {
		q = q.Where("`group` = ?", enums.ExpenseGroupHotelCheckInOut)
	}

	// (parent with a child of childType) OR (type = childType)
	q = q.Where(
		h.DB.Where(
			h.DB.Where("type = ?", enums.CashCategoryParent).
				Where("EXISTS (SELECT 1 FROM cash_categories ch WHERE ch.parent_id = cash_categories.id AND ch.type = ?)", childType),
		).Or("type = ?", childType),
	)

	var cats []models.CashCategory
	q.Order("name").Find(&cats)

	data := make([]gin.H, 0, len(cats))
	for _, cat := range cats {
		data = append(data, gin.H{
			"id":        cat.ID,
			"parent_id": cat.ParentID,
			"group":     cat.Group,
			"type":      cat.Type,
			"name":      cat.Name,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// WalletStore mirrors WalletController::store.
func (h *Handler) WalletStore(c *gin.Context) {
	userID := h.principal(c).User.ID

	typ := c.PostForm("type")
	amount := c.PostForm("amount")
	currency := c.PostForm("currency")
	details := c.PostForm("details")
	date := c.PostForm("date")
	groupID := c.PostForm("group_id")
	categoryID := c.PostForm("category_id")
	toUserID := c.PostForm("to_user_id")

	errs := map[string][]string{}
	if typ != "income" && typ != "expense" && typ != "transfer" {
		errs["type"] = []string{"The selected type is invalid."}
	}
	if amount == "" {
		errs["amount"] = []string{"The amount field is required."}
	}
	if currency != "SAR" && currency != "IDR" {
		errs["currency"] = []string{"The selected currency is invalid."}
	}
	if details == "" {
		errs["details"] = []string{"The details field is required."}
	}
	if date == "" {
		errs["date"] = []string{"The date field is required."}
	}
	if (typ == "income" || typ == "expense") && categoryID == "" {
		errs["category_id"] = []string{"The category id field is required when type is " + typ + "."}
	}
	if typ == "transfer" && toUserID == "" {
		errs["to_user_id"] = []string{"The to user id field is required when type is transfer."}
	}
	if len(errs) > 0 {
		validationError(c, errs)
		return
	}

	typeMap := map[string]string{
		"income":   enums.UserCashIncome,
		"expense":  enums.UserCashExpense,
		"transfer": enums.UserCashTransfer,
	}

	var attachmentKey *string
	if fh, err := c.FormFile("attachment"); err == nil && fh != nil {
		content, contentType, ext, err := readUpload(fh)
		if err == nil {
			if key, err := h.Storage.Store(c.Request.Context(), "attachments", ext, contentType, content); err == nil {
				attachmentKey = &key
			}
		}
	}

	txn := models.UserCash{
		UserID:   userID,
		Type:     typeMap[typ],
		Currency: currency,
		Details:  &details,
	}
	txn.Amount = parseFloat(amount)
	if t := parseDate(date); t != nil {
		txn.CashedAt = t
	}
	if groupID != "" {
		txn.GroupID = parseUintPtr(groupID)
	}
	if categoryID != "" {
		txn.CategoryID = parseUintPtr(categoryID)
	}
	if toUserID != "" {
		txn.ToUserID = parseUintPtr(toUserID)
	}
	if attachmentKey != nil {
		txn.Attachments = jsonArray([]string{*attachmentKey})
	}

	if err := h.DB.Create(&txn).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not create transaction."})
		return
	}

	h.DB.Preload("Group.Customer").Preload("Category.Parent").First(&txn, txn.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Transaction created successfully",
		"data":    h.transformTransaction(&txn, userID),
	})
}

func (h *Handler) transformTransaction(trx *models.UserCash, userID uint64) gin.H {
	typ := ""
	switch trx.Type {
	case enums.UserCashIncome:
		typ = "income"
	case enums.UserCashExpense:
		typ = "expense"
	case enums.UserCashTransfer:
		if trx.ToUserID != nil && *trx.ToUserID == userID {
			typ = "transfer_in"
		} else {
			typ = "transfer_out"
		}
	}

	var groupName *string
	if trx.Group != nil {
		n := trx.Group.FullName()
		groupName = &n
	}
	var categoryName *string
	if trx.Category != nil {
		n := trx.Category.FullName()
		categoryName = &n
	}

	return gin.H{
		"id":            itoa(trx.ID),
		"amount":        trx.Amount,
		"currency":      trx.Currency,
		"exchange_rate": trx.ExchangeRate,
		"type":          typ,
		"group":         groupName,
		"category":      categoryName,
		"details":       trx.Details,
		"date":          support.ISO(trx.CashedAt),
		"attachments":   h.attachmentURLs(trx.Attachments),
	}
}

// attachmentURLs mirrors UserCash::attachments_urls (map paths to S3 urls).
func (h *Handler) attachmentURLs(raw datatypes.JSON) []string {
	var paths []string
	decodeJSON(raw, &paths)
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != "" {
			out = append(out, h.Storage.URL(p))
		}
	}
	return out
}
