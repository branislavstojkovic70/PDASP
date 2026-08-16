package contract

import "github.com/hyperledger/fabric-contract-api-go/v2/contractapi"

// Initial world state.
//
// The assignment asks for at least 2 merchants with at least 2 products each plus
// a few customers. There are 4 and 13 here, with deliberately varied data because
// the CouchDB rich queries are demonstrated against it:
//
//   - "Milk 2.8% 1L" (supermarket) and "Baby milk formula 400g" (pharmacy) share a
//     term in the name across two merchant types, so a query combining
//     $regex "milk" with $in ["SUPERMARKET","PHARMACY"] returns both
//   - prices span 69.50 to 89900.00, so a price range has something to cut
//   - some products have an expiry date and some do not, for the date range query
//   - customer C004 deliberately has very little money, giving the insufficient
//     funds test a stable case

var initialMerchantTypes = []MerchantType{
	{Code: "SUPERMARKET", Name: "Supermarket", Description: "Groceries and everyday goods"},
	{Code: "AUTO_PARTS", Name: "Auto parts", Description: "Spare parts and vehicle equipment"},
	{Code: "PHARMACY", Name: "Pharmacy", Description: "Medicines, supplements and medical supplies"},
	{Code: "ELECTRONICS", Name: "Electronics store", Description: "Appliances and consumer electronics"},
	{Code: "CONSTRUCTION", Name: "Construction materials", Description: "Building materials and tools"},
}

var initialMerchants = []MerchantInput{
	{MerchantId: "M001", Name: "Maxi Center", Type: "SUPERMARKET", TaxId: "100200300", OpeningBalance: 50000},
	{MerchantId: "M002", Name: "AutoPart Plus", Type: "AUTO_PARTS", TaxId: "100200301", OpeningBalance: 35000},
	{MerchantId: "M003", Name: "Health Pharmacy", Type: "PHARMACY", TaxId: "100200302", OpeningBalance: 20000},
	{MerchantId: "M004", Name: "TechnoMarket", Type: "ELECTRONICS", TaxId: "100200303", OpeningBalance: 80000},
}

var initialProducts = map[string][]ProductInput{
	"M001": {
		{Code: "P001", Name: "Milk 2.8% 1L", ExpiryDate: "2026-09-30", Price: 129.99, Quantity: 250},
		{Code: "P002", Name: "White bread 500g", ExpiryDate: "2026-08-20", Price: 69.50, Quantity: 120},
		{Code: "P003", Name: "Ground coffee 200g", ExpiryDate: "2027-03-15", Price: 389.00, Quantity: 80},
		{Code: "P004", Name: "Eggs class A 10 pcs", ExpiryDate: "2026-09-05", Price: 249.99, Quantity: 60},
	},
	"M002": {
		{Code: "P005", Name: "Oil filter", Price: 1450.00, Quantity: 40},
		{Code: "P006", Name: "Battery 60Ah", Price: 12900.00, Quantity: 12},
		{Code: "P007", Name: "Motor oil 5W30 4L", ExpiryDate: "2029-01-01", Price: 4790.00, Quantity: 25},
	},
	"M003": {
		{Code: "P008", Name: "Vitamin C 1000mg", ExpiryDate: "2027-06-30", Price: 890.00, Quantity: 100},
		{Code: "P009", Name: "Painkiller 20 tablets", ExpiryDate: "2026-12-31", Price: 320.50, Quantity: 200},
		{Code: "P010", Name: "Baby milk formula 400g", ExpiryDate: "2026-10-15", Price: 1590.00, Quantity: 35},
	},
	"M004": {
		{Code: "P011", Name: "Wireless headphones", Price: 7990.00, Quantity: 30},
		{Code: "P012", Name: "Laptop 14 inch", Price: 89900.00, Quantity: 8},
		{Code: "P013", Name: "Microwave oven", Price: 15490.00, Quantity: 15},
	},
}

var initialCustomers = []CustomerInput{
	{CustomerId: "C001", FirstName: "Petar", LastName: "Petrovic", Email: "petar.petrovic@example.com", OpeningBalance: 25000},
	{CustomerId: "C002", FirstName: "Ana", LastName: "Anic", Email: "ana.anic@example.com", OpeningBalance: 5000},
	{CustomerId: "C003", FirstName: "Marko", LastName: "Markovic", Email: "marko.markovic@example.com", OpeningBalance: 150000},
	{CustomerId: "C004", FirstName: "Jelena", LastName: "Jelic", Email: "jelena.jelic@example.com", OpeningBalance: 300},
}

// InitReport summarises what the initialization wrote.
type InitReport struct {
	MerchantTypes   int `json:"merchantTypes"`
	Merchants       int `json:"merchants"`
	Products        int `json:"products"`
	Customers       int `json:"customers"`
	SkippedExisting int `json:"skippedExisting"`
}

// InitLedger sets up the initial world state.
//
// It is called as a normal invoke transaction after the chaincode definition is
// committed. Existing records are skipped rather than failing the whole call, so
// InitLedger can safely be run again, for example after a chaincode upgrade on the
// same channel.
func (c *TradeContract) InitLedger(
	ctx contractapi.TransactionContextInterface,
) (*InitReport, error) {

	report := &InitReport{}

	for _, merchantType := range initialMerchantTypes {
		exists, err := stateExists(ctx, merchantTypeKey(merchantType.Code))
		if err != nil {
			return nil, err
		}
		if exists {
			report.SkippedExisting++
			continue
		}
		if _, err := c.CreateMerchantType(ctx, merchantType.Code, merchantType.Name,
			merchantType.Description); err != nil {
			return nil, err
		}
		report.MerchantTypes++
	}

	for _, input := range initialMerchants {
		exists, err := stateExists(ctx, merchantKey(input.MerchantId))
		if err != nil {
			return nil, err
		}
		if exists {
			report.SkippedExisting++
			continue
		}
		if _, err := c.CreateMerchant(ctx, input.MerchantId, input.Name, input.Type,
			input.TaxId, input.OpeningBalance); err != nil {
			return nil, err
		}
		report.Merchants++

		for _, product := range initialProducts[input.MerchantId] {
			if _, err := c.AddProduct(ctx, input.MerchantId, product.Code, product.Name,
				product.ExpiryDate, product.Price, product.Quantity); err != nil {
				return nil, err
			}
			report.Products++
		}
	}

	for _, input := range initialCustomers {
		exists, err := stateExists(ctx, customerKey(input.CustomerId))
		if err != nil {
			return nil, err
		}
		if exists {
			report.SkippedExisting++
			continue
		}
		if _, err := c.CreateCustomer(ctx, input.CustomerId, input.FirstName,
			input.LastName, input.Email, input.OpeningBalance); err != nil {
			return nil, err
		}
		report.Customers++
	}

	return report, nil
}
