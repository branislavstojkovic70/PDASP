package contract

import "strings"

// World state key prefixes.
//
// Merchant "M001" and customer "M001" are different entities; without a prefix
// they would share a key and overwrite each other. The prefix also enables
// GetStateByRange over a single entity type (see readByPrefix), which is the only
// form of lookup that would also work on LevelDB.
const (
	prefixMerchantType = "TYPE_"
	prefixMerchant     = "MERCHANT_"
	prefixProduct      = "PRODUCT_"
	prefixCustomer     = "CUSTOMER_"
	prefixInvoice      = "INVOICE_"
)

func merchantTypeKey(code string) string { return prefixMerchantType + code }
func merchantKey(id string) string       { return prefixMerchant + id }
func productKey(code string) string      { return prefixProduct + code }
func customerKey(id string) string       { return prefixCustomer + id }
func invoiceKey(id string) string        { return prefixInvoice + id }

// prefixRange returns the bounds for GetStateByRange over a single prefix. The
// upper bound is exclusive, so the last character of the prefix is incremented:
// "MERCHANT_" .. "MERCHANT`" covers every key starting with "MERCHANT_".
func prefixRange(prefix string) (string, string) {
	if prefix == "" {
		return "", ""
	}
	last := prefix[len(prefix)-1]
	upper := prefix[:len(prefix)-1] + string(last+1)
	return prefix, upper
}

// stripPrefix returns the bare identifier from a world state key.
func stripPrefix(key, prefix string) string {
	return strings.TrimPrefix(key, prefix)
}
