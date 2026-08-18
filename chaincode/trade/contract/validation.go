package contract

import (
	"strings"
	"time"
)

func requireText(name, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errEmptyParameter(name)
	}
	return trimmed, nil
}

func requirePositiveAmount(name string, value float64) error {
	if value <= 0 {
		return errNonPositiveAmount(name, value)
	}
	return nil
}

func requirePositiveQuantity(name string, value int) error {
	if value <= 0 {
		return errNonPositiveQuantity(name, value)
	}
	return nil
}

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


func roundMoney(value float64) float64 {
	return float64(int64(value*100+signOf(value)*0.5)) / 100
}

func signOf(value float64) float64 {
	if value < 0 {
		return -1
	}
	return 1
}
