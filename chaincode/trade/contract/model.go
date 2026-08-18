package contract

const (
	DocMerchantType = "merchantType"
	DocMerchant     = "merchant"
	DocProduct      = "product"
	DocCustomer     = "customer"
	DocInvoice      = "invoice"
)


type MerchantType struct {
	DocType     string `json:"docType"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Merchant struct {
	DocType    string   `json:"docType"`
	MerchantId string   `json:"merchantId"`
	Name       string   `json:"name"`
	Type       string   `json:"type"` 
	TaxId      string   `json:"taxId"`
	Products   []string `json:"products"`
	Invoices   []string `json:"invoices"` 
	Balance    float64  `json:"balance"`
}


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
	Date        string  `json:"date"` 
}
