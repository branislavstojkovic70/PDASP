package contract

import "github.com/hyperledger/fabric-contract-api-go/v2/contractapi"


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

type InitReport struct {
	MerchantTypes   int `json:"merchantTypes"`
	Merchants       int `json:"merchants"`
	Products        int `json:"products"`
	Customers       int `json:"customers"`
	SkippedExisting int `json:"skippedExisting"`
}


func (c *TradeContract) InitLedger(
	ctx contractapi.TransactionContextInterface,
) (*InitReport, error) {

	report := &InitReport{}

	knownTypes := map[string]bool{}
	for _, merchantType := range initialMerchantTypes {
		knownTypes[merchantType.Code] = true

		key := merchantTypeKey(merchantType.Code)
		exists, err := stateExists(ctx, key)
		if err != nil {
			return nil, err
		}
		if exists {
			report.SkippedExisting++
			continue
		}
		if err := writeState(ctx, key, &MerchantType{
			DocType:     DocMerchantType,
			Code:        merchantType.Code,
			Name:        merchantType.Name,
			Description: merchantType.Description,
		}); err != nil {
			return nil, err
		}
		report.MerchantTypes++
	}

	for _, input := range initialMerchants {
		key := merchantKey(input.MerchantId)
		exists, err := stateExists(ctx, key)
		if err != nil {
			return nil, err
		}
		if exists {
			report.SkippedExisting++
			continue
		}
		if !knownTypes[input.Type] {
			return nil, errMerchantTypeNotFound(input.Type)
		}

		merchant := &Merchant{
			DocType:    DocMerchant,
			MerchantId: input.MerchantId,
			Name:       input.Name,
			Type:       input.Type,
			TaxId:      input.TaxId,
			Products:   []string{},
			Invoices:   []string{},
			Balance:    roundMoney(input.OpeningBalance),
		}

		// Products first, so the merchant document is written once with its
		// product list already complete.
		for _, product := range initialProducts[input.MerchantId] {
			expiry, err := parseExpiryDate(product.ExpiryDate)
			if err != nil {
				return nil, err
			}
			if err := writeState(ctx, productKey(product.Code), &Product{
				DocType:      DocProduct,
				Code:         product.Code,
				Name:         product.Name,
				ExpiryDate:   expiry,
				Price:        roundMoney(product.Price),
				Quantity:     product.Quantity,
				MerchantId:   merchant.MerchantId,
				MerchantType: merchant.Type,
			}); err != nil {
				return nil, err
			}
			merchant.Products = append(merchant.Products, product.Code)
			report.Products++
		}

		if err := writeState(ctx, key, merchant); err != nil {
			return nil, err
		}
		report.Merchants++
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
