package contract

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// Rich queries against the CouchDB state database.
//
// This is the part that makes the assignment require CouchDB instead of the
// default LevelDB. LevelDB is a pure key-value store: the value is an opaque byte
// array, so it supports only GetState(key) and GetStateByRange(from, to). CouchDB
// stores the value as a JSON document and can filter on its fields, so a search by
// name, price or merchant type is expressed as a single selector.
//
// Worth stating on the defense: the result of a rich query is NOT re-validated at
// commit time (the read set holds only keys, not the query predicate), so these
// queries are used exclusively as queries (evaluateTransaction), never as the
// basis for a write.

// SearchCriteria are the parameters of the combined product search.
//
// The fields are pointers because the difference between "not supplied" and
// "supplied as zero" matters: PriceFrom = 0 is a legitimate filter, while the
// absence of a filter is something else entirely.
type SearchCriteria struct {
	Name          *string  `json:"name,omitempty"`          // substring, case insensitive
	Code          *string  `json:"code,omitempty"`          // substring of the code
	MerchantTypes []string `json:"merchantTypes,omitempty"` // any of these
	MerchantId    *string  `json:"merchantId,omitempty"`
	PriceFrom     *float64 `json:"priceFrom,omitempty"`
	PriceTo       *float64 `json:"priceTo,omitempty"`
	ExpiryFrom    *string  `json:"expiryFrom,omitempty"` // YYYY-MM-DD
	ExpiryTo      *string  `json:"expiryTo,omitempty"`
	InStockOnly   bool     `json:"inStockOnly,omitempty"`
	SortByPrice   string   `json:"sortByPrice,omitempty"` // "asc" | "desc" | ""
}

// ---------------------------------------------------------------------------
// Single category searches (an assignment requirement)
// ---------------------------------------------------------------------------

// SearchProductsByName returns products whose name contains the given text.
func (c *TradeContract) SearchProductsByName(
	ctx contractapi.TransactionContextInterface, name string,
) ([]*Product, error) {
	name, err := requireText("name", name)
	if err != nil {
		return nil, err
	}
	return c.SearchProducts(ctx, mustJSON(SearchCriteria{Name: &name}))
}

// SearchProductsByCode returns products whose code contains the given text.
// For an exact match use ReadProduct, which reads straight from the key.
func (c *TradeContract) SearchProductsByCode(
	ctx contractapi.TransactionContextInterface, code string,
) ([]*Product, error) {
	code, err := requireText("code", code)
	if err != nil {
		return nil, err
	}
	return c.SearchProducts(ctx, mustJSON(SearchCriteria{Code: &code}))
}

// SearchProductsByMerchantType returns products of merchants in a given line of
// business.
//
// This is the query that best shows why the denormalization pays off: the merchant
// type sits on the product document, so one selector is enough. Without it you
// would first have to find all merchants of that type and then filter products by
// their ids.
func (c *TradeContract) SearchProductsByMerchantType(
	ctx contractapi.TransactionContextInterface, merchantType string,
) ([]*Product, error) {
	merchantType, err := requireText("merchantType", merchantType)
	if err != nil {
		return nil, err
	}
	return c.SearchProducts(ctx, mustJSON(SearchCriteria{
		MerchantTypes: []string{merchantType},
	}))
}

// SearchProductsByPrice returns products within a price range.
func (c *TradeContract) SearchProductsByPrice(
	ctx contractapi.TransactionContextInterface, priceFrom float64, priceTo float64,
) ([]*Product, error) {
	if priceFrom < 0 || priceTo < 0 {
		return nil, errNonPositiveAmount("priceFrom/priceTo", min(priceFrom, priceTo))
	}
	if priceTo > 0 && priceFrom > priceTo {
		return nil, errInvertedRange(priceFrom, priceTo)
	}
	criteria := SearchCriteria{PriceFrom: &priceFrom}
	if priceTo > 0 {
		criteria.PriceTo = &priceTo
	}
	return c.SearchProducts(ctx, mustJSON(criteria))
}

// GetMerchantProducts returns every product a merchant currently offers.
func (c *TradeContract) GetMerchantProducts(
	ctx contractapi.TransactionContextInterface, merchantId string,
) ([]*Product, error) {
	merchantId, err := requireText("merchantId", merchantId)
	if err != nil {
		return nil, err
	}
	if _, err := loadMerchant(ctx, merchantId); err != nil {
		return nil, err
	}
	return c.SearchProducts(ctx, mustJSON(SearchCriteria{MerchantId: &merchantId}))
}

// ProductsExpiringBefore returns products that expire before a date and are still
// in stock.
//
// The expiry date is stored as YYYY-MM-DD, a format whose lexicographic order
// matches chronological order, which is why $lte over a string behaves like a date
// comparison. The $gt:"" bound excludes products with no expiry date (empty string).
func (c *TradeContract) ProductsExpiringBefore(
	ctx contractapi.TransactionContextInterface, beforeDate string,
) ([]*Product, error) {
	beforeDate, err := parseExpiryDate(beforeDate)
	if err != nil {
		return nil, err
	}
	if beforeDate == "" {
		return nil, errEmptyParameter("beforeDate")
	}
	empty := ""
	return c.SearchProducts(ctx, mustJSON(SearchCriteria{
		ExpiryFrom:  &empty,
		ExpiryTo:    &beforeDate,
		InStockOnly: true,
		SortByPrice: "asc",
	}))
}

// ---------------------------------------------------------------------------
// Combined search
// ---------------------------------------------------------------------------

// SearchProducts is the combined search over any combination of categories, which
// the assignment requires ("by each category separately or by any combination").
//
// The selector is assembled dynamically, adding only the conditions that were
// supplied, so the same code serves a search on one field and a search on all six.
func (c *TradeContract) SearchProducts(
	ctx contractapi.TransactionContextInterface, criteriaJSON string,
) ([]*Product, error) {

	criteria, err := parseCriteria(criteriaJSON)
	if err != nil {
		return nil, err
	}

	query, err := buildProductQuery(*criteria)
	if err != nil {
		return nil, err
	}

	products, err := runQuery[Product](ctx, query)
	if err != nil {
		return nil, err
	}
	sortByPrice(products, criteria.SortByPrice)
	return products, nil
}

// PagedResult is one page of results together with the bookmark for the next one.
type PagedResult struct {
	Products     []*Product `json:"products"`
	FetchedCount int        `json:"fetchedCount"`
	NextBookmark string     `json:"nextBookmark"`
}

// SearchProductsPaged is the same search with pagination.
//
// GetQueryResultWithPagination is only allowed in query transactions; Fabric
// rejects it inside an invoke, because the number of keys read would depend on the
// page size and the read set would not be reproducible.
func (c *TradeContract) SearchProductsPaged(
	ctx contractapi.TransactionContextInterface,
	criteriaJSON string, pageSize int, bookmark string,
) (*PagedResult, error) {

	if err := requirePositiveQuantity("pageSize", pageSize); err != nil {
		return nil, err
	}
	criteria, err := parseCriteria(criteriaJSON)
	if err != nil {
		return nil, err
	}
	query, err := buildProductQuery(*criteria)
	if err != nil {
		return nil, err
	}

	iterator, metadata, err := ctx.GetStub().GetQueryResultWithPagination(
		query, int32(pageSize), bookmark)
	if err != nil {
		return nil, errQuery(err)
	}
	defer iterator.Close()

	products, err := readIterator[Product](iterator)
	if err != nil {
		return nil, err
	}
	sortByPrice(products, criteria.SortByPrice)

	return &PagedResult{
		Products:     products,
		FetchedCount: int(metadata.FetchedRecordsCount),
		NextBookmark: metadata.Bookmark,
	}, nil
}

// ---------------------------------------------------------------------------
// Invoice queries
// ---------------------------------------------------------------------------

// CustomerInvoices returns one customer's invoices, newest first.
func (c *TradeContract) CustomerInvoices(
	ctx contractapi.TransactionContextInterface, customerId string,
) ([]*Invoice, error) {
	customerId, err := requireText("customerId", customerId)
	if err != nil {
		return nil, err
	}
	if _, err := loadCustomer(ctx, customerId); err != nil {
		return nil, err
	}
	return invoicesByField(ctx, "customerId", customerId)
}

// MerchantInvoices returns one merchant's invoices, newest first.
func (c *TradeContract) MerchantInvoices(
	ctx contractapi.TransactionContextInterface, merchantId string,
) ([]*Invoice, error) {
	merchantId, err := requireText("merchantId", merchantId)
	if err != nil {
		return nil, err
	}
	if _, err := loadMerchant(ctx, merchantId); err != nil {
		return nil, err
	}
	return invoicesByField(ctx, "merchantId", merchantId)
}

// CustomerInvoicesAbove returns a customer's invoices above a given amount. It
// combines an equality and a range condition on two different fields in one
// selector.
func (c *TradeContract) CustomerInvoicesAbove(
	ctx contractapi.TransactionContextInterface, customerId string, minAmount float64,
) ([]*Invoice, error) {
	customerId, err := requireText("customerId", customerId)
	if err != nil {
		return nil, err
	}
	if err := requirePositiveAmount("minAmount", minAmount); err != nil {
		return nil, err
	}

	query := mustJSON(map[string]any{
		"selector": map[string]any{
			"docType":    DocInvoice,
			"customerId": customerId,
			"total":      map[string]any{"$gte": minAmount},
		},
	})
	invoices, err := runQuery[Invoice](ctx, query)
	if err != nil {
		return nil, err
	}
	sortInvoices(invoices)
	return invoices, nil
}

func invoicesByField(
	ctx contractapi.TransactionContextInterface, field, value string,
) ([]*Invoice, error) {
	query := mustJSON(map[string]any{
		"selector": map[string]any{"docType": DocInvoice, field: value},
	})
	invoices, err := runQuery[Invoice](ctx, query)
	if err != nil {
		return nil, err
	}
	sortInvoices(invoices)
	return invoices, nil
}

// ---------------------------------------------------------------------------
// Selector assembly
// ---------------------------------------------------------------------------

func buildProductQuery(criteria SearchCriteria) (string, error) {
	selector := map[string]any{"docType": DocProduct}

	if criteria.Name != nil {
		if err := checkSearchTerm(*criteria.Name); err != nil {
			return "", err
		}
		selector["name"] = map[string]any{"$regex": "(?i)" + regexp.QuoteMeta(*criteria.Name)}
	}
	if criteria.Code != nil {
		if err := checkSearchTerm(*criteria.Code); err != nil {
			return "", err
		}
		selector["code"] = map[string]any{"$regex": "(?i)" + regexp.QuoteMeta(*criteria.Code)}
	}
	if len(criteria.MerchantTypes) > 0 {
		selector["merchantType"] = map[string]any{"$in": criteria.MerchantTypes}
	}
	if criteria.MerchantId != nil && *criteria.MerchantId != "" {
		selector["merchantId"] = *criteria.MerchantId
	}

	// Bounds are merged into one object so $gte and $lte apply to the same field.
	if price := boundsOf(criteria.PriceFrom, criteria.PriceTo); price != nil {
		selector["price"] = price
	}
	if expiry := boundsOf(criteria.ExpiryFrom, criteria.ExpiryTo); expiry != nil {
		selector["expiryDate"] = expiry
	}
	if criteria.InStockOnly {
		selector["quantity"] = map[string]any{"$gt": 0}
	}

	return mustJSON(map[string]any{"selector": selector}), nil
}

// boundsOf builds {"$gte":from,"$lte":to}, skipping bounds that were not supplied.
// An empty string as the lower bound becomes $gt instead, so documents without a
// value (products with no expiry date) are excluded rather than included.
func boundsOf[T float64 | string](from, to *T) map[string]any {
	if from == nil && to == nil {
		return nil
	}
	result := map[string]any{}
	if from != nil {
		if text, isString := any(*from).(string); isString && text == "" {
			result["$gt"] = ""
		} else {
			result["$gte"] = *from
		}
	}
	if to != nil {
		result["$lte"] = *to
	}
	return result
}

func parseCriteria(criteriaJSON string) (*SearchCriteria, error) {
	trimmed := strings.TrimSpace(criteriaJSON)
	if trimmed == "" || trimmed == "{}" {
		return &SearchCriteria{}, nil
	}
	var criteria SearchCriteria
	if err := json.Unmarshal([]byte(trimmed), &criteria); err != nil {
		return nil, errInvalidJSON("the search criteria", err)
	}
	return &criteria, nil
}

// checkSearchTerm rejects overly long search terms. The regex is built from user
// input, so the content goes through QuoteMeta; the length limit keeps a query
// from loading CouchDB for no reason.
func checkSearchTerm(term string) error {
	if len(term) > 100 {
		return errSearchTermTooLong(len(term), 100)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Query execution
// ---------------------------------------------------------------------------

func runQuery[T any](
	ctx contractapi.TransactionContextInterface, query string,
) ([]*T, error) {
	iterator, err := ctx.GetStub().GetQueryResult(query)
	if err != nil {
		return nil, errQuery(err)
	}
	defer iterator.Close()
	return readIterator[T](iterator)
}

// ---------------------------------------------------------------------------
// Sorting
//
// Sorting happens in Go rather than through a CouchDB "sort" clause: CouchDB
// rejects a sort on a field that is not covered by an index, so the query would
// start failing depending on whether the META-INF indexes had been applied yet.
// Sorting here is also deterministic across endorsers.
// ---------------------------------------------------------------------------

func sortByPrice(products []*Product, direction string) {
	switch strings.ToLower(direction) {
	case "asc":
		sort.SliceStable(products, func(i, j int) bool {
			if products[i].Price == products[j].Price {
				return products[i].Code < products[j].Code
			}
			return products[i].Price < products[j].Price
		})
	case "desc":
		sort.SliceStable(products, func(i, j int) bool {
			if products[i].Price == products[j].Price {
				return products[i].Code < products[j].Code
			}
			return products[i].Price > products[j].Price
		})
	default:
		sort.SliceStable(products, func(i, j int) bool {
			return products[i].Code < products[j].Code
		})
	}
}

// sortInvoices orders invoices from newest to oldest.
func sortInvoices(invoices []*Invoice) {
	sort.SliceStable(invoices, func(i, j int) bool {
		if invoices[i].Date == invoices[j].Date {
			return invoices[i].Id < invoices[j].Id
		}
		return invoices[i].Date > invoices[j].Date
	})
}

// mustJSON serializes a value that cannot fail by construction (maps of strings
// and primitive types).
func mustJSON(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}
