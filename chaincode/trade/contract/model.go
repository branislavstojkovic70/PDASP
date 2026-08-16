// Package contract implements the chaincode of the trading system.
//
// Every state is stored in the world state as a JSON document. Each document
// carries a docType field: without it there would be no way to write a CouchDB
// selector that returns only products or only invoices, because all types share
// the same database.
package contract

// Values of the DocType field. They are also part of every CouchDB selector, so
// they must stay stable: changing a value invalidates the existing indexes.
const (
	DocMerchantType = "merchantType"
	DocMerchant     = "merchant"
	DocProduct      = "product"
	DocCustomer     = "customer"
	DocInvoice      = "invoice"
)

// MerchantType is the catalogue of merchant lines of business (supermarket, auto
// parts, ...). The assignment requires merchant types to be kept as state on the
// blockchain rather than hard coded in the application.
type MerchantType struct {
	DocType     string `json:"docType"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Merchant sells products to customers.
//
// Products and Invoices hold identifiers only, not embedded objects. Otherwise
// every purchase would rewrite the entire merchant document including all of its
// products, creating needless conflicts in the read-write set whenever two
// transactions touch the same merchant.
type Merchant struct {
	DocType    string   `json:"docType"`
	MerchantId string   `json:"merchantId"`
	Name       string   `json:"name"`
	Type       string   `json:"type"` // MerchantType.Code
	TaxId      string   `json:"taxId"`
	Products   []string `json:"products"` // product codes on offer
	Invoices   []string `json:"invoices"` // ids of issued invoices
	Balance    float64  `json:"balance"`
}

// Product is an item offered by one merchant.
//
// MerchantType is deliberately denormalized from the merchant document. The
// assignment requires searching products by merchant type; without this field such
// a search would need two CouchDB queries and a join in code, whereas here it is a
// single selector. ChangeMerchantType keeps it consistent by propagating a type
// change to all of the merchant's products.
type Product struct {
	DocType      string  `json:"docType"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	ExpiryDate   string  `json:"expiryDate"` // YYYY-MM-DD, empty when not applicable
	Price        float64 `json:"price"`
	Quantity     int     `json:"quantity"`
	MerchantId   string  `json:"merchantId"`
	MerchantType string  `json:"merchantType"`
}

// Customer buys products from merchants.
type Customer struct {
	DocType    string   `json:"docType"`
	CustomerId string   `json:"customerId"`
	FirstName  string   `json:"firstName"`
	LastName   string   `json:"lastName"`
	Email      string   `json:"email"`
	Invoices   []string `json:"invoices"`
	Balance    float64  `json:"balance"`
}

// Invoice records a completed purchase and links a customer to a merchant.
//
// ProductName, UnitPrice and Quantity are snapshots taken at purchase time: the
// product is deleted from the world state once its quantity reaches zero, so it
// could not be read back from the invoice later.
type Invoice struct {
	DocType     string  `json:"docType"`
	Id          string  `json:"id"`
	MerchantId  string  `json:"merchantId"`
	CustomerId  string  `json:"customerId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unitPrice"`
	Total       float64 `json:"total"`
	Date        string  `json:"date"` // RFC3339, from TxTimestamp
}
