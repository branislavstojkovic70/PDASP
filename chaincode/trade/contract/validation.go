package contract

import (
	"strings"
	"time"
)

// Input validation. Every chaincode function validates its arguments before it
// touches the world state: partially written state cannot be rolled back, so it
// would stay in the ledger forever.

// requireText checks that a string is not empty (or whitespace only) and returns
// it trimmed.
func requireText(name, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errEmptyParameter(name)
	}
	return trimmed, nil
}

// requirePositiveAmount validates a monetary amount (price, deposit).
func requirePositiveAmount(name string, value float64) error {
	if value <= 0 {
		return errNonPositiveAmount(name, value)
	}
	return nil
}

// requirePositiveQuantity validates an integer quantity.
func requirePositiveQuantity(name string, value int) error {
	if value <= 0 {
		return errNonPositiveQuantity(name, value)
	}
	return nil
}

// parseExpiryDate validates the YYYY-MM-DD format. An empty value is allowed,
// since the assignment says "expiry date (if it has one)".
//
// The format is deliberately lexicographically sortable: CouchDB compares strings
// byte by byte, so a selector such as {"expiryDate":{"$lte":"2026-01-01"}} behaves
// like a date comparison only as long as every date uses this shape.
func parseExpiryDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", trimmed); err != nil {
		return "", errInvalidDate(value)
	}
	return trimmed, nil
}

// requireEmail wants a non empty string that looks like an email address. The
// check is deliberately loose: full RFC 5322 validation inside chaincode is
// pointless because the address is never verified by sending anything to it.
func requireEmail(value string) (string, error) {
	email, err := requireText("email", value)
	if err != nil {
		return "", err
	}
	if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") ||
		strings.HasSuffix(email, "@") {
		return "", errEmptyParameter("email (must contain @)")
	}
	return email, nil
}

// roundMoney reduces a monetary amount to two decimal places.
//
// Adding and subtracting float64 values accumulates error (0.1 + 0.2 is not
// exactly 0.3), so after several purchases a balance would drift away from the sum
// of the individual invoices. Rounding after every operation keeps that in check.
func roundMoney(value float64) float64 {
	return float64(int64(value*100+signOf(value)*0.5)) / 100
}

func signOf(value float64) float64 {
	if value < 0 {
		return -1
	}
	return 1
}
