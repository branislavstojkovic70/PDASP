package contract

import "strings"


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
