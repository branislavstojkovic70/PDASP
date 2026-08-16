package contract

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Initialization
// ---------------------------------------------------------------------------

func TestInitLedgerWritesInitialState(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	report, err := tradeContract.InitLedger(ctx)
	if err != nil {
		t.Fatalf("second InitLedger failed: %v", err)
	}
	// The second call must not write anything, everything already exists.
	if report.Merchants != 0 || report.Customers != 0 || report.MerchantTypes != 0 {
		t.Errorf("second InitLedger wrote new records: %+v", report)
	}

	merchants, err := tradeContract.GetAllMerchants(ctx)
	if err != nil {
		t.Fatalf("GetAllMerchants: %v", err)
	}
	if len(merchants) != 4 {
		t.Errorf("expected 4 merchants, got %d", len(merchants))
	}

	// The assignment requires at least 2 merchants with at least 2 products each.
	for _, merchant := range merchants {
		if len(merchant.Products) < 2 {
			t.Errorf("merchant %s has only %d products, the minimum is 2",
				merchant.MerchantId, len(merchant.Products))
		}
	}

	customers, err := tradeContract.GetAllCustomers(ctx)
	if err != nil {
		t.Fatalf("GetAllCustomers: %v", err)
	}
	if len(customers) != 4 {
		t.Errorf("expected 4 customers, got %d", len(customers))
	}

	types, err := tradeContract.GetAllMerchantTypes(ctx)
	if err != nil {
		t.Fatalf("GetAllMerchantTypes: %v", err)
	}
	if len(types) != 5 {
		t.Errorf("expected 5 merchant types, got %d", len(types))
	}
}

// ---------------------------------------------------------------------------
// Creating entities
// ---------------------------------------------------------------------------

func TestCreateMerchantRejectsDuplicate(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	_, err := tradeContract.CreateMerchant(ctx, "M001", "Copy", "SUPERMARKET", "999", 0)
	if err == nil {
		t.Fatal("expected a duplicate error, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestCreateMerchantRejectsUnknownType(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	_, err := tradeContract.CreateMerchant(ctx, "M999", "New shop", "NO_SUCH_TYPE", "123", 100)
	if err == nil {
		t.Fatal("expected an unknown type error, got nil")
	}
	if !strings.Contains(err.Error(), "catalogue") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestCreateMerchantRejectsEmptyParameters(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	cases := []struct {
		label                         string
		id, name, merchantType, taxId string
	}{
		{"empty id", "  ", "Shop", "SUPERMARKET", "123"},
		{"empty name", "M900", "", "SUPERMARKET", "123"},
		{"empty type", "M901", "Shop", "", "123"},
		{"empty tax id", "M902", "Shop", "SUPERMARKET", "   "},
	}

	for _, testCase := range cases {
		t.Run(testCase.label, func(t *testing.T) {
			if _, err := tradeContract.CreateMerchant(ctx, testCase.id, testCase.name,
				testCase.merchantType, testCase.taxId, 0); err == nil {
				t.Fatal("expected a validation error, got nil")
			}
		})
	}
}

func TestCreateCustomersIsAllOrNothing(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	// The second customer has an invalid email, so neither may be stored.
	input := `[
		{"customerId":"C900","firstName":"New","lastName":"Customer","email":"new@example.com","openingBalance":100},
		{"customerId":"C901","firstName":"Second","lastName":"Customer","email":"no-at-sign","openingBalance":100}
	]`
	if _, err := tradeContract.CreateCustomers(ctx, input); err == nil {
		t.Fatal("expected an invalid email error, got nil")
	}

	// C901 never reached the state. Fabric discards the whole transaction proposal
	// when chaincode returns an error, so nothing is written to the ledger.
	if _, err := tradeContract.ReadCustomer(ctx, "C901"); err == nil {
		t.Error("C901 must not exist")
	}
}

func TestAddProductsUpdatesMerchantOnce(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	input := `[
		{"code":"P900","name":"Test item A","price":100,"quantity":5},
		{"code":"P901","name":"Test item B","price":200,"quantity":5}
	]`
	added, err := tradeContract.AddProducts(ctx, "M001", input)
	if err != nil {
		t.Fatalf("AddProducts: %v", err)
	}
	if len(added) != 2 {
		t.Fatalf("expected 2 products, got %d", len(added))
	}

	merchant, err := tradeContract.ReadMerchant(ctx, "M001")
	if err != nil {
		t.Fatalf("ReadMerchant: %v", err)
	}
	if !contains(merchant.Products, "P900") || !contains(merchant.Products, "P901") {
		t.Errorf("new codes were not recorded on the merchant: %v", merchant.Products)
	}

	// The merchant type must be copied onto the product (denormalization).
	if added[0].MerchantType != "SUPERMARKET" {
		t.Errorf("merchantType was not denormalized: %q", added[0].MerchantType)
	}
}

func TestAddProductRejectsUnknownMerchant(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	_, err := tradeContract.AddProduct(ctx, "M999", "P999", "Item", "", 100, 1)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected a merchant not found error, got: %v", err)
	}
}

func TestAddProductRejectsInvalidExpiryDate(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	_, err := tradeContract.AddProduct(ctx, "M001", "P999", "Item", "31.12.2026", 100, 1)
	if err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("expected a date format error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Purchase
// ---------------------------------------------------------------------------

func TestBuyProductSucceeds(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	customerBefore, _ := tradeContract.ReadCustomer(ctx, "C001")
	merchantBefore, _ := tradeContract.ReadMerchant(ctx, "M001")
	productBefore, _ := tradeContract.ReadProduct(ctx, "P001")

	result, err := tradeContract.BuyProduct(ctx, "C001", "M001", "P001", 3)
	if err != nil {
		t.Fatalf("BuyProduct: %v", err)
	}

	expectedTotal := roundMoney(productBefore.Price * 3)
	if result.Invoice.Total != expectedTotal {
		t.Errorf("total: expected %.2f, got %.2f", expectedTotal, result.Invoice.Total)
	}
	if result.CustomerBalance != roundMoney(customerBefore.Balance-expectedTotal) {
		t.Errorf("customer balance: expected %.2f, got %.2f",
			roundMoney(customerBefore.Balance-expectedTotal), result.CustomerBalance)
	}
	if result.MerchantBalance != roundMoney(merchantBefore.Balance+expectedTotal) {
		t.Errorf("merchant balance: expected %.2f, got %.2f",
			roundMoney(merchantBefore.Balance+expectedTotal), result.MerchantBalance)
	}
	if result.RemainingQuantity != productBefore.Quantity-3 {
		t.Errorf("remaining quantity: expected %d, got %d",
			productBefore.Quantity-3, result.RemainingQuantity)
	}

	// The invoice must be linked to both participants.
	customerAfter, _ := tradeContract.ReadCustomer(ctx, "C001")
	merchantAfter, _ := tradeContract.ReadMerchant(ctx, "M001")
	if !contains(customerAfter.Invoices, result.Invoice.Id) {
		t.Error("invoice was not added to the customer")
	}
	if !contains(merchantAfter.Invoices, result.Invoice.Id) {
		t.Error("invoice was not added to the merchant")
	}

	// The date must come from the mock stub TxTimestamp, not the machine clock.
	if result.Invoice.Date != "2026-03-01T12:00:00Z" {
		t.Errorf("date did not come from TxTimestamp: %q", result.Invoice.Date)
	}
	if result.Invoice.Id != "tx-0001" {
		t.Errorf("invoice id is not the TxID: %q", result.Invoice.Id)
	}
}

func TestBuyProductRejectsInsufficientFunds(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	// C004 has 300, the laptop costs 89900.
	_, err := tradeContract.BuyProduct(ctx, "C004", "M004", "P012", 1)
	if err == nil {
		t.Fatal("expected an insufficient funds error, got nil")
	}
	if !strings.Contains(err.Error(), "insufficient funds") {
		t.Errorf("unexpected message: %v", err)
	}

	// Nothing may have changed.
	customer, _ := tradeContract.ReadCustomer(ctx, "C004")
	if customer.Balance != 300 {
		t.Errorf("customer balance changed to %.2f", customer.Balance)
	}
	product, _ := tradeContract.ReadProduct(ctx, "P012")
	if product.Quantity != 8 {
		t.Errorf("product quantity changed to %d", product.Quantity)
	}
}

func TestBuyProductRejectsInsufficientStock(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	_, err := tradeContract.BuyProduct(ctx, "C003", "M004", "P012", 100)
	if err == nil {
		t.Fatal("expected an insufficient stock error, got nil")
	}
	if !strings.Contains(err.Error(), "insufficient stock") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestBuyProductDeletesProductWhenStockHitsZero(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	// P012 (laptop) has 8 units; C003 has 150000 while the laptop costs 89900, so
	// the price is lowered to make buying all 8 units affordable.
	if _, err := tradeContract.UpdatePrice(ctx, "P012", 100); err != nil {
		t.Fatalf("UpdatePrice: %v", err)
	}

	result, err := tradeContract.BuyProduct(ctx, "C003", "M004", "P012", 8)
	if err != nil {
		t.Fatalf("BuyProduct: %v", err)
	}
	if !result.ProductRemoved {
		t.Error("product was not flagged as removed")
	}

	if _, err := tradeContract.ReadProduct(ctx, "P012"); err == nil {
		t.Error("product is still in the world state")
	}

	merchant, _ := tradeContract.ReadMerchant(ctx, "M004")
	if contains(merchant.Products, "P012") {
		t.Errorf("code remained in the merchant offering: %v", merchant.Products)
	}

	// The invoice must still carry the name and price of the deleted product.
	invoice, err := tradeContract.ReadInvoice(ctx, result.Invoice.Id)
	if err != nil {
		t.Fatalf("ReadInvoice: %v", err)
	}
	if invoice.ProductName != "Laptop 14 inch" {
		t.Errorf("invoice lost the product name: %q", invoice.ProductName)
	}
}

func TestBuyProductRejectsProductOfAnotherMerchant(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	// P001 belongs to M001, not M002.
	_, err := tradeContract.BuyProduct(ctx, "C001", "M002", "P001", 1)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "is not offered by merchant") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestBuyProductRejectsUnknownEntities(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	cases := []struct {
		label                       string
		customer, merchant, product string
		expectedFragment            string
	}{
		{"unknown customer", "C999", "M001", "P001", "customer with id 'C999'"},
		{"unknown merchant", "C001", "M999", "P001", "merchant with id 'M999'"},
		{"unknown product", "C001", "M001", "P999", "product with code 'P999'"},
	}

	for _, testCase := range cases {
		t.Run(testCase.label, func(t *testing.T) {
			_, err := tradeContract.BuyProduct(ctx, testCase.customer, testCase.merchant,
				testCase.product, 1)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), testCase.expectedFragment) {
				t.Errorf("message does not mention %q: %v", testCase.expectedFragment, err)
			}
		})
	}
}

func TestBuyProductRejectsNonPositiveQuantity(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	for _, quantity := range []int{0, -1} {
		if _, err := tradeContract.BuyProduct(ctx, "C001", "M001", "P001", quantity); err == nil {
			t.Errorf("quantity %d passed without an error", quantity)
		}
	}
}

// ---------------------------------------------------------------------------
// Deposit
// ---------------------------------------------------------------------------

func TestDeposit(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	result, err := tradeContract.Deposit(ctx, "customer", "C002", 1500.55)
	if err != nil {
		t.Fatalf("Deposit (customer): %v", err)
	}
	if result.NewBalance != 6500.55 {
		t.Errorf("new customer balance: expected 6500.55, got %.2f", result.NewBalance)
	}

	result, err = tradeContract.Deposit(ctx, "MERCHANT", "M002", 500)
	if err != nil {
		t.Fatalf("Deposit (merchant, upper case): %v", err)
	}
	if result.NewBalance != 35500 {
		t.Errorf("new merchant balance: expected 35500, got %.2f", result.NewBalance)
	}
}

func TestDepositRejectsInvalidInput(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	if _, err := tradeContract.Deposit(ctx, "bank", "C001", 100); err == nil {
		t.Error("an unknown entity type passed")
	}
	if _, err := tradeContract.Deposit(ctx, "customer", "C001", -100); err == nil {
		t.Error("a negative amount passed")
	}
	if _, err := tradeContract.Deposit(ctx, "customer", "C001", 0); err == nil {
		t.Error("a zero amount passed")
	}
	if _, err := tradeContract.Deposit(ctx, "customer", "C999", 100); err == nil {
		t.Error("a deposit to an unknown customer passed")
	}
}

// ---------------------------------------------------------------------------
// Searches (CouchDB rich queries)
// ---------------------------------------------------------------------------

func TestSearchByNameIsCaseInsensitive(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	// "Milk 2.8% 1L" (supermarket) and "Baby milk formula 400g" (pharmacy).
	products, err := tradeContract.SearchProductsByName(ctx, "MILK")
	if err != nil {
		t.Fatalf("SearchProductsByName: %v", err)
	}
	if joined(productCodes(products)) != "P001,P010" {
		t.Errorf("expected P001,P010, got %v", productCodes(products))
	}
}

func TestSearchByMerchantType(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	products, err := tradeContract.SearchProductsByMerchantType(ctx, "AUTO_PARTS")
	if err != nil {
		t.Fatalf("SearchProductsByMerchantType: %v", err)
	}
	// Without an explicit sortByPrice the result is ordered by code.
	if joined(productCodes(products)) != "P005,P006,P007" {
		t.Errorf("expected P005,P006,P007 (sorted by code), got %v", productCodes(products))
	}
}

func TestSearchByPrice(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	products, err := tradeContract.SearchProductsByPrice(ctx, 100, 400)
	if err != nil {
		t.Fatalf("SearchProductsByPrice: %v", err)
	}
	for _, product := range products {
		if product.Price < 100 || product.Price > 400 {
			t.Errorf("product %s priced %.2f is outside the range 100-400",
				product.Code, product.Price)
		}
	}
	if len(products) == 0 {
		t.Error("the range 100-400 returned no products")
	}
}

func TestSearchByPriceRejectsInvertedRange(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	if _, err := tradeContract.SearchProductsByPrice(ctx, 500, 100); err == nil {
		t.Error("an inverted range passed without an error")
	}
}

func TestCombinedSearch(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	// Name contains "milk", merchant type is supermarket or pharmacy, price
	// 1000-2000. Only P010 (1590.00) qualifies: P001 costs 129.99 and would
	// otherwise match, so the lower bound gives the price filter something to cut.
	criteria := `{
		"name":"milk",
		"merchantTypes":["SUPERMARKET","PHARMACY"],
		"priceFrom":1000,
		"priceTo":2000,
		"sortByPrice":"asc"
	}`
	products, err := tradeContract.SearchProducts(ctx, criteria)
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if joined(productCodes(products)) != "P010" {
		t.Errorf("expected P010, got %v", productCodes(products))
	}

	// Without the price filter both must come back.
	products, err = tradeContract.SearchProducts(ctx,
		`{"name":"milk","merchantTypes":["SUPERMARKET","PHARMACY"],"sortByPrice":"asc"}`)
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if joined(productCodes(products)) != "P001,P010" {
		t.Errorf("expected P001,P010 (ascending by price), got %v", productCodes(products))
	}
}

func TestCombinedSearchWithoutCriteriaReturnsEverything(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	for _, input := range []string{"", "{}", "  "} {
		products, err := tradeContract.SearchProducts(ctx, input)
		if err != nil {
			t.Fatalf("SearchProducts(%q): %v", input, err)
		}
		if len(products) != 13 {
			t.Errorf("SearchProducts(%q): expected 13, got %d", input, len(products))
		}
	}
}

func TestCombinedSearchRejectsInvalidJSON(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	if _, err := tradeContract.SearchProducts(ctx, `{"name":`); err == nil {
		t.Error("invalid JSON passed without an error")
	}
}

func TestSortByPriceDescending(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	products, err := tradeContract.SearchProducts(ctx, `{"sortByPrice":"desc"}`)
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	for i := 1; i < len(products); i++ {
		if products[i-1].Price < products[i].Price {
			t.Fatalf("result is not sorted descending at position %d: %.2f before %.2f",
				i, products[i-1].Price, products[i].Price)
		}
	}
}

func TestProductsExpiringBefore(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	products, err := tradeContract.ProductsExpiringBefore(ctx, "2026-10-01")
	if err != nil {
		t.Fatalf("ProductsExpiringBefore: %v", err)
	}

	// P002 (2026-08-20), P004 (2026-09-05) and P001 (2026-09-30). Products without
	// an expiry date (P005, P006, P011, P012, P013) must NOT appear.
	if joined(productCodes(products)) != "P002,P001,P004" {
		t.Errorf("expected P002,P001,P004 (ascending by price), got %v", productCodes(products))
	}
	for _, product := range products {
		if product.ExpiryDate == "" {
			t.Errorf("a product without an expiry date (%s) leaked into the result", product.Code)
		}
	}
}

func TestGetMerchantProducts(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	products, err := tradeContract.GetMerchantProducts(ctx, "M003")
	if err != nil {
		t.Fatalf("GetMerchantProducts: %v", err)
	}
	if len(products) != 3 {
		t.Errorf("expected 3 products for M003, got %d", len(products))
	}

	if _, err := tradeContract.GetMerchantProducts(ctx, "M999"); err == nil {
		t.Error("a query for an unknown merchant passed without an error")
	}
}

func TestPagedSearch(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	first, err := tradeContract.SearchProductsPaged(ctx, "{}", 5, "")
	if err != nil {
		t.Fatalf("SearchProductsPaged: %v", err)
	}
	if first.FetchedCount != 5 {
		t.Errorf("first page: expected 5 records, got %d", first.FetchedCount)
	}
	if first.NextBookmark == "" {
		t.Fatal("first page returned no bookmark for the next one")
	}

	second, err := tradeContract.SearchProductsPaged(ctx, "{}", 5, first.NextBookmark)
	if err != nil {
		t.Fatalf("SearchProductsPaged (second page): %v", err)
	}
	if second.FetchedCount != 5 {
		t.Errorf("second page: expected 5 records, got %d", second.FetchedCount)
	}

	// The pages must not overlap.
	for _, firstProduct := range first.Products {
		for _, secondProduct := range second.Products {
			if firstProduct.Code == secondProduct.Code {
				t.Errorf("product %s appears on both pages", firstProduct.Code)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Invoices
// ---------------------------------------------------------------------------

func TestCustomerAndMerchantInvoices(t *testing.T) {
	ctx, stub, tradeContract := seededLedger(t)

	if _, err := tradeContract.BuyProduct(ctx, "C001", "M001", "P001", 2); err != nil {
		t.Fatalf("first purchase: %v", err)
	}
	// Every transaction has its own TxID; without changing it the second invoice
	// would overwrite the first.
	stub.txID = "tx-0002"
	if _, err := tradeContract.BuyProduct(ctx, "C001", "M001", "P003", 1); err != nil {
		t.Fatalf("second purchase: %v", err)
	}

	invoices, err := tradeContract.CustomerInvoices(ctx, "C001")
	if err != nil {
		t.Fatalf("CustomerInvoices: %v", err)
	}
	if len(invoices) != 2 {
		t.Errorf("expected 2 invoices, got %d", len(invoices))
	}

	invoices, err = tradeContract.MerchantInvoices(ctx, "M001")
	if err != nil {
		t.Fatalf("MerchantInvoices: %v", err)
	}
	if len(invoices) != 2 {
		t.Errorf("expected 2 invoices for the merchant, got %d", len(invoices))
	}

	// Amount filter: 2 x 129.99 = 259.98, while the coffee costs 389.00.
	expensive, err := tradeContract.CustomerInvoicesAbove(ctx, "C001", 300)
	if err != nil {
		t.Fatalf("CustomerInvoicesAbove: %v", err)
	}
	if len(expensive) != 1 || expensive[0].ProductCode != "P003" {
		t.Errorf("expected only the P003 invoice, got %d invoices", len(expensive))
	}
}

func TestReadInvoiceForUnknownId(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	if _, err := tradeContract.ReadInvoice(ctx, "no-such-id"); err == nil {
		t.Error("expected an error for an unknown invoice")
	}
}

// ---------------------------------------------------------------------------
// Changing a merchant type
// ---------------------------------------------------------------------------

func TestChangeMerchantTypePropagatesToProducts(t *testing.T) {
	ctx, _, tradeContract := seededLedger(t)

	if _, err := tradeContract.ChangeMerchantType(ctx, "M001", "CONSTRUCTION"); err != nil {
		t.Fatalf("ChangeMerchantType: %v", err)
	}

	products, err := tradeContract.GetMerchantProducts(ctx, "M001")
	if err != nil {
		t.Fatalf("GetMerchantProducts: %v", err)
	}
	for _, product := range products {
		if product.MerchantType != "CONSTRUCTION" {
			t.Errorf("product %s kept the old type %q", product.Code, product.MerchantType)
		}
	}

	// Searching by the new type must find them.
	matched, err := tradeContract.SearchProductsByMerchantType(ctx, "CONSTRUCTION")
	if err != nil {
		t.Fatalf("SearchProductsByMerchantType: %v", err)
	}
	if len(matched) != 4 {
		t.Errorf("expected 4 products under CONSTRUCTION, got %d", len(matched))
	}

	if _, err := tradeContract.ChangeMerchantType(ctx, "M001", "NO_SUCH_TYPE"); err == nil {
		t.Error("a change to an unknown type passed without an error")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestRoundMoney(t *testing.T) {
	cases := []struct {
		input, expected float64
	}{
		{0.1 + 0.2, 0.3},
		{129.994, 129.99},
		{129.995, 130.00},
		{-5.555, -5.56},
		{1000, 1000},
	}
	for _, testCase := range cases {
		if got := roundMoney(testCase.input); got != testCase.expected {
			t.Errorf("roundMoney(%v) = %v, expected %v", testCase.input, got, testCase.expected)
		}
	}
}

func TestPrefixRange(t *testing.T) {
	from, to := prefixRange(prefixMerchant)
	if from != "MERCHANT_" || to != "MERCHANT`" {
		t.Errorf("prefixRange returned (%q, %q)", from, to)
	}

	// The bounds must cover every prefixed key and nothing else.
	if !("MERCHANT_M001" >= from && "MERCHANT_M001" < to) {
		t.Error("a prefixed key falls outside the range")
	}
	if "MERCHANTX" >= from && "MERCHANTX" < to {
		t.Error("a key without the prefix fell inside the range")
	}
}

func TestAppendUniqueAndRemoveValue(t *testing.T) {
	list := []string{"a", "b", "c"}

	if appendUnique(list, "b"); len(list) != 3 {
		t.Error("appendUnique added a duplicate")
	}
	extended := appendUnique(list, "d")
	if len(extended) != 4 {
		t.Errorf("appendUnique did not add the new value: %v", extended)
	}

	shortened, removed := removeValue(list, "b")
	if !removed || joined(shortened) != "a,c" {
		t.Errorf("removeValue returned %v (removed=%v)", shortened, removed)
	}
	// The original list must be untouched, removeValue copies.
	if joined(list) != "a,b,c" {
		t.Errorf("removeValue modified the original list: %v", list)
	}

	if _, removed := removeValue(list, "z"); removed {
		t.Error("removeValue reported removing a value that was not present")
	}
}

func TestParseExpiryDate(t *testing.T) {
	if expiry, err := parseExpiryDate("  "); err != nil || expiry != "" {
		t.Errorf("an empty expiry date must be allowed, got (%q, %v)", expiry, err)
	}
	if _, err := parseExpiryDate("2026-13-01"); err == nil {
		t.Error("a non existent month passed")
	}
	if expiry, err := parseExpiryDate(" 2026-12-31 "); err != nil || expiry != "2026-12-31" {
		t.Errorf("date with surrounding spaces returned (%q, %v)", expiry, err)
	}
}
