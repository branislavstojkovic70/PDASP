package contract

import "fmt"


func errMerchantNotFound(id string) error {
	return fmt.Errorf("merchant with id '%s' does not exist", id)
}

func errCustomerNotFound(id string) error {
	return fmt.Errorf("customer with id '%s' does not exist", id)
}

func errProductNotFound(code string) error {
	return fmt.Errorf("product with code '%s' does not exist", code)
}

func errInvoiceNotFound(id string) error {
	return fmt.Errorf("invoice with id '%s' does not exist", id)
}

func errMerchantTypeNotFound(code string) error {
	return fmt.Errorf("merchant type '%s' is not in the catalogue", code)
}


func errMerchantExists(id string) error {
	return fmt.Errorf("merchant with id '%s' already exists", id)
}

func errCustomerExists(id string) error {
	return fmt.Errorf("customer with id '%s' already exists", id)
}

func errProductExists(code string) error {
	return fmt.Errorf("product with code '%s' already exists", code)
}

func errMerchantTypeExists(code string) error {
	return fmt.Errorf("merchant type '%s' is already in the catalogue", code)
}


func errInsufficientFunds(customerId string, balance, required float64) error {
	return fmt.Errorf(
		"customer '%s' has insufficient funds: balance is %.2f but %.2f is required",
		customerId, balance, required)
}

func errInsufficientStock(code string, inStock, requested int) error {
	return fmt.Errorf(
		"product '%s' has insufficient stock: %d available but %d requested",
		code, inStock, requested)
}

func errProductNotSoldByMerchant(code, merchantId string) error {
	return fmt.Errorf("product '%s' is not offered by merchant '%s'", code, merchantId)
}

func errUnknownEntityType(entityType string) error {
	return fmt.Errorf("unknown entity type '%s': expected 'merchant' or 'customer'", entityType)
}


func errEmptyParameter(name string) error {
	return fmt.Errorf("parameter '%s' is required and must not be empty", name)
}

func errNonPositiveAmount(name string, value float64) error {
	return fmt.Errorf("parameter '%s' must be greater than zero, got %.2f", name, value)
}

func errNonPositiveQuantity(name string, value int) error {
	return fmt.Errorf("parameter '%s' must be greater than zero, got %d", name, value)
}

func errInvalidDate(value string) error {
	return fmt.Errorf("expiry date '%s' is not in YYYY-MM-DD format", value)
}

func errInvalidJSON(what string, err error) error {
	return fmt.Errorf("invalid JSON for %s: %w", what, err)
}

func errDuplicateInBatch(what, id string) error {
	return fmt.Errorf("%s '%s' appears more than once in the same request", what, id)
}

func errInvertedRange(from, to float64) error {
	return fmt.Errorf("lower bound of the range (%.2f) is greater than the upper bound (%.2f)", from, to)
}

func errSearchTermTooLong(length, allowed int) error {
	return fmt.Errorf("search term is too long: %d characters, the limit is %d", length, allowed)
}


func errRead(key string, err error) error {
	return fmt.Errorf("reading key '%s' from the world state failed: %w", key, err)
}

func errWrite(key string, err error) error {
	return fmt.Errorf("writing key '%s' to the world state failed: %w", key, err)
}

func errDelete(key string, err error) error {
	return fmt.Errorf("deleting key '%s' from the world state failed: %w", key, err)
}

func errQuery(err error) error {
	return fmt.Errorf("executing a query against the world state failed: %w", err)
}

func errTxTimestamp(err error) error {
	return fmt.Errorf("reading the transaction timestamp failed: %w", err)
}
