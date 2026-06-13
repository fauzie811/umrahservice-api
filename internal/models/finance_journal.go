package models

import (
	"time"

	"gorm.io/gorm"

	"umrahservice-api/internal/enums"
)

// Chart-of-account codes, mirroring config/finance.php `coa`. Only the codes
// referenced by UserCash journaling are included.
const (
	coaMainCash       = "1.101.001"
	coaAdvanceCash    = "1.101.003"
	coaDefaultExpense = "6.000.009"

	// baseCurrency is the ledger reporting currency for exchange-rate lookups.
	baseCurrency = "IDR"

	// morphUserCash is the polymorphic alias for UserCash (Laravel morph map).
	morphUserCash = "user_cash"
)

// JournalEntry maps `journal_entries` (double-entry header, polymorphic owner).
type JournalEntry struct {
	ID              uint64    `gorm:"primaryKey"`
	TransactionType *string   `gorm:"column:transaction_type"`
	TransactionID   *uint64   `gorm:"column:transaction_id"`
	EntryDate       time.Time `gorm:"column:entry_date"`
	Details         *string   `gorm:"column:details"`
	Memo            *string   `gorm:"column:memo"`
	CreatedByID     *uint64   `gorm:"column:created_by_id"`
	UpdatedByID     *uint64   `gorm:"column:updated_by_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (JournalEntry) TableName() string { return "journal_entries" }

// deleteWithItems removes the entry and its items, mirroring Laravel's
// overridden JournalEntry::delete() which cascades to items first.
func (e *JournalEntry) deleteWithItems(tx *gorm.DB) error {
	if err := tx.Where("entry_id = ?", e.ID).Delete(&JournalEntryItem{}).Error; err != nil {
		return err
	}
	return tx.Delete(e).Error
}

// JournalEntryItem maps `journal_entry_items` (one debit/credit line).
type JournalEntryItem struct {
	ID           uint64    `gorm:"primaryKey"`
	EntryID      uint64    `gorm:"column:entry_id"`
	AccountID    *uint64   `gorm:"column:account_id"`
	Type         string    `gorm:"column:type"` // d|c
	CurrencyCode string    `gorm:"column:currency_code"`
	ExchangeRate float64   `gorm:"column:exchange_rate"`
	Amount       float64   `gorm:"column:amount"`
	OwnerType    *string   `gorm:"column:owner_type"`
	OwnerID      *uint64   `gorm:"column:owner_id"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (JournalEntryItem) TableName() string { return "journal_entry_items" }

// CashAccount maps `cash_accounts` (chart of accounts).
type CashAccount struct {
	ID   uint64 `gorm:"primaryKey"`
	Code string `gorm:"column:code"`
}

func (CashAccount) TableName() string { return "cash_accounts" }

// ExchangeRate maps `exchange_rates` (date-keyed conversion rates).
type ExchangeRate struct {
	ID           uint64    `gorm:"primaryKey"`
	RateDate     time.Time `gorm:"column:rate_date"`
	BaseCurrency string    `gorm:"column:base_currency"`
	Currency     string    `gorm:"column:currency"`
	ExchangeRate float64   `gorm:"column:exchange_rate"`
}

func (ExchangeRate) TableName() string { return "exchange_rates" }

// cashAccountIDForCode mirrors CashAccount::getIdForCode.
func cashAccountIDForCode(tx *gorm.DB, code string) *uint64 {
	var ids []uint64
	tx.Model(&CashAccount{}).Where("code = ?", code).Limit(1).Pluck("id", &ids)
	if len(ids) == 0 || ids[0] == 0 {
		return nil
	}
	return &ids[0]
}

// getExchangeRate mirrors ExchangeRate::getExchangeRate: latest rate within the
// 7 days up to `date`, falling back to the per-currency defaults.
func getExchangeRate(tx *gorm.DB, date time.Time, base, to string) float64 {
	if base == to {
		return 1
	}

	from := date.AddDate(0, 0, -7).Format("2006-01-02")
	until := date.Format("2006-01-02")

	var rates []float64
	tx.Model(&ExchangeRate{}).
		Where("base_currency = ? AND currency = ?", base, to).
		Where("rate_date >= ? AND rate_date <= ?", from, until).
		Order("rate_date DESC").
		Limit(1).
		Pluck("exchange_rate", &rates)

	if len(rates) > 0 {
		return rates[0]
	}
	switch to {
	case "USD":
		return 16000
	case "SAR":
		return 4300
	default:
		return 1
	}
}

// AfterSave mirrors Laravel UserCash::booted() static::saved, syncing the
// double-entry journal for this transaction. syncRelated (Bill-payment sync) is
// intentionally omitted: the Go wallet never sets related_type and Bill has no
// Go model.
func (u *UserCash) AfterSave(tx *gorm.DB) error {
	return u.syncJournalEntry(tx)
}

// syncJournalEntry mirrors UserCash::syncJournalEntry (app/Models/Finance/UserCash.php).
func (u *UserCash) syncJournalEntry(tx *gorm.DB) error {
	// Load the category (account_id + group) when present.
	category := u.Category
	if category == nil && u.CategoryID != nil {
		var c CashCategory
		if err := tx.First(&c, *u.CategoryID).Error; err == nil {
			category = &c
		}
	}

	// Locate any existing entry for this transaction.
	var existing JournalEntry
	hasExisting := tx.Where("transaction_type = ? AND transaction_id = ?", morphUserCash, u.ID).
		First(&existing).Error == nil

	// Skip + delete when fixed, or a vendor-payment tied to a related record.
	vendorPayment := category != nil && category.Group != nil && *category.Group == enums.ExpenseGroupVendorPayment
	if u.IsFixed || (vendorPayment && u.RelatedID != nil) {
		if hasExisting {
			return existing.deleteWithItems(tx)
		}
		return nil
	}

	var cashedAt time.Time
	if u.CashedAt != nil {
		cashedAt = *u.CashedAt
	} else {
		cashedAt = time.Now()
	}

	// Upsert the entry header.
	entry := existing
	entry.EntryDate = cashedAt
	entry.Details = u.Details
	if hasExisting {
		if err := tx.Model(&entry).Updates(map[string]any{
			"entry_date":    cashedAt,
			"details":       u.Details,
			"updated_by_id": u.UserID,
		}).Error; err != nil {
			return err
		}
	} else {
		txType := morphUserCash
		entry = JournalEntry{
			TransactionType: &txType,
			TransactionID:   &u.ID,
			EntryDate:       cashedAt,
			Details:         u.Details,
			CreatedByID:     &u.UserID,
			UpdatedByID:     &u.UserID,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
	}

	rate := getExchangeRate(tx, cashedAt, baseCurrency, u.Currency)
	advanceID := cashAccountIDForCode(tx, coaAdvanceCash)

	switch u.Type {
	case enums.UserCashIncome:
		accountID := category.accountIDOr(cashAccountIDForCode(tx, coaMainCash))
		if err := upsertItem(tx, entry.ID, "d", advanceID, u.Currency, rate, u.Amount, ownerUser, &u.UserID); err != nil {
			return err
		}
		if err := upsertItem(tx, entry.ID, "c", accountID, u.Currency, rate, u.Amount, nil, nil); err != nil {
			return err
		}
	case enums.UserCashExpense:
		accountID := category.accountIDOr(cashAccountIDForCode(tx, coaDefaultExpense))
		if err := upsertItem(tx, entry.ID, "d", accountID, u.Currency, rate, u.Amount, nil, nil); err != nil {
			return err
		}
		if err := upsertItem(tx, entry.ID, "c", advanceID, u.Currency, rate, u.Amount, ownerUser, &u.UserID); err != nil {
			return err
		}
	case enums.UserCashTransfer:
		if err := upsertItem(tx, entry.ID, "d", advanceID, u.Currency, rate, u.Amount, ownerUser, u.ToUserID); err != nil {
			return err
		}
		if err := upsertItem(tx, entry.ID, "c", advanceID, u.Currency, rate, u.Amount, ownerUser, &u.UserID); err != nil {
			return err
		}
	}

	return nil
}

// ownerUser is the polymorphic owner_type for user-owned journal lines.
var ownerUser = ptr("user")

// accountIDOr returns the category's account_id, or the provided fallback when
// the category is nil or has no account.
func (c *CashCategory) accountIDOr(fallback *uint64) *uint64 {
	if c != nil && c.AccountID != nil {
		return c.AccountID
	}
	return fallback
}

// upsertItem mirrors JournalEntry::items()->updateOrCreate(['type' => ...], [...]),
// matching one line per (entry, type).
func upsertItem(tx *gorm.DB, entryID uint64, typ string, accountID *uint64, currency string, rate, amount float64, ownerType *string, ownerID *uint64) error {
	values := map[string]any{
		"account_id":    accountID,
		"currency_code": currency,
		"exchange_rate": rate,
		"amount":        amount,
		"owner_type":    ownerType,
		"owner_id":      ownerID,
	}

	var item JournalEntryItem
	err := tx.Where("entry_id = ? AND type = ?", entryID, typ).First(&item).Error
	if err == nil {
		return tx.Model(&item).Updates(values).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	return tx.Create(&JournalEntryItem{
		EntryID:      entryID,
		Type:         typ,
		AccountID:    accountID,
		CurrencyCode: currency,
		ExchangeRate: rate,
		Amount:       amount,
		OwnerType:    ownerType,
		OwnerID:      ownerID,
	}).Error
}

func ptr(s string) *string { return &s }
