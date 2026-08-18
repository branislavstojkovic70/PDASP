package contract

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

type SearchCriteria struct {
	Name          *string  `json:"name,omitempty"`         
	Code          *string  `json:"code,omitempty"`         
	MerchantTypes []string `json:"merchantTypes,omitempty"` 
	MerchantId    *string  `json:"merchantId,omitempty"`
	PriceFrom     *float64 `json:"priceFrom,omitempty"`
	PriceTo       *float64 `json:"priceTo,omitempty"`
	ExpiryFrom    *string  `json:"expiryFrom,omitempty"` 
	ExpiryTo      *string  `json:"expiryTo,omitempty"`
	InStockOnly   bool     `json:"inStockOnly,omitempty"`
	SortByPrice   string   `json:"sortByPrice,omitempty"`
}


func (c *TradeContract) SearchProductsByName(
	ctx contractapi.TransactionContextInterface, name string,
) ([]*Product, error) {
	name, err := requireText("name", name)
	if err != nil {
		return nil, err
	}
	return c.SearchProducts(ctx, mustJSON(SearchCriteria{Name: &name}))
}


func (c *TradeContract) SearchProductsByCode(
	ctx contractapi.TransactionContextInterface, code string,
) ([]*Product, error) {
	code, err := requireText("code", code)
	if err != nil {
		return nil, err
	}
	return c.SearchProducts(ctx, mustJSON(SearchCriteria{Code: &code}))
}


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

type PagedResult struct {
	Products     []*Product `json:"products"`
	FetchedCount int        `json:"fetchedCount"`
	NextBookmark string     `json:"nextBookmark"`
}

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

func checkSearchTerm(term string) error {
	if len(term) > 100 {
		return errSearchTermTooLong(len(term), 100)
	}
	return nil
}

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

func sortInvoices(invoices []*Invoice) {
	sort.SliceStable(invoices, func(i, j int) bool {
		if invoices[i].Date == invoices[j].Date {
			return invoices[i].Id < invoices[j].Id
		}
		return invoices[i].Date > invoices[j].Date
	})
}

func mustJSON(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}
