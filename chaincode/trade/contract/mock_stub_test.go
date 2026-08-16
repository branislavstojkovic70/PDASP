package contract

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/queryresult"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// A fake ChaincodeStub backed by an in-memory world state.
//
// Instead of a mocking library this is a real in-memory ledger with a simplified
// CouchDB Mango evaluator. The reason: half of the logic under test lives in the
// selectors assembled by queries.go, and a mock that returns a canned response
// would never establish whether a selector is correct at all.
//
// Only the subset of Mango operators the chaincode actually uses is supported:
// equality, $regex, $in, $gte, $gt, $lte and $lt.

type mockStub struct {
	shim.ChaincodeStubInterface // the remaining methods are not needed in tests

	state map[string][]byte
	txID  string
	now   time.Time
}

func newStub() *mockStub {
	return &mockStub{
		state: map[string][]byte{},
		txID:  "tx-0001",
		now:   time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
	}
}

// newContext builds a transaction context over a fresh mock stub.
func newContext() (contractapi.TransactionContextInterface, *mockStub) {
	stub := newStub()
	ctx := new(contractapi.TransactionContext)
	ctx.SetStub(stub)
	return ctx, stub
}

func (s *mockStub) GetState(key string) ([]byte, error) {
	return s.state[key], nil
}

func (s *mockStub) PutState(key string, value []byte) error {
	stored := make([]byte, len(value))
	copy(stored, value)
	s.state[key] = stored
	return nil
}

func (s *mockStub) DelState(key string) error {
	delete(s.state, key)
	return nil
}

func (s *mockStub) GetTxID() string { return s.txID }

func (s *mockStub) GetTxTimestamp() (*timestamppb.Timestamp, error) {
	return timestamppb.New(s.now), nil
}

func (s *mockStub) GetStateByRange(from, to string) (shim.StateQueryIteratorInterface, error) {
	var matched []string
	for key := range s.state {
		if key >= from && (to == "" || key < to) {
			matched = append(matched, key)
		}
	}
	sort.Strings(matched)
	return s.iterator(matched), nil
}

func (s *mockStub) GetQueryResult(query string) (shim.StateQueryIteratorInterface, error) {
	keys, err := s.keysMatching(query)
	if err != nil {
		return nil, err
	}
	return s.iterator(keys), nil
}

func (s *mockStub) GetQueryResultWithPagination(
	query string, pageSize int32, bookmark string,
) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	keys, err := s.keysMatching(query)
	if err != nil {
		return nil, nil, err
	}

	start := 0
	if bookmark != "" {
		for i, key := range keys {
			if key == bookmark {
				start = i
				break
			}
		}
	}
	end := min(start+int(pageSize), len(keys))
	page := keys[start:end]

	next := ""
	if end < len(keys) {
		next = keys[end]
	}

	return s.iterator(page), &peer.QueryResponseMetadata{
		FetchedRecordsCount: int32(len(page)),
		Bookmark:            next,
	}, nil
}

func (s *mockStub) keysMatching(query string) ([]string, error) {
	var parsed struct {
		Selector map[string]any `json:"selector"`
	}
	if err := json.Unmarshal([]byte(query), &parsed); err != nil {
		return nil, fmt.Errorf("invalid query %q: %w", query, err)
	}

	var matched []string
	for key, bytes := range s.state {
		var document map[string]any
		if err := json.Unmarshal(bytes, &document); err != nil {
			continue
		}
		if matchesSelector(document, parsed.Selector) {
			matched = append(matched, key)
		}
	}
	sort.Strings(matched)
	return matched, nil
}

func (s *mockStub) iterator(keys []string) shim.StateQueryIteratorInterface {
	return &mockIterator{stub: s, keys: keys}
}

// ---------------------------------------------------------------------------

type mockIterator struct {
	stub *mockStub
	keys []string
	next int
}

func (i *mockIterator) HasNext() bool { return i.next < len(i.keys) }

func (i *mockIterator) Next() (*queryresult.KV, error) {
	if !i.HasNext() {
		return nil, fmt.Errorf("iterator is exhausted")
	}
	key := i.keys[i.next]
	i.next++
	return &queryresult.KV{Key: key, Value: i.stub.state[key]}, nil
}

func (i *mockIterator) Close() error { return nil }

// ---------------------------------------------------------------------------
// Simplified Mango evaluator
// ---------------------------------------------------------------------------

func matchesSelector(document, selector map[string]any) bool {
	for field, condition := range selector {
		if !matchesCondition(document[field], condition) {
			return false
		}
	}
	return true
}

func matchesCondition(value, condition any) bool {
	operators, isMap := condition.(map[string]any)
	if !isMap {
		return valuesEqual(value, condition)
	}

	for operator, expected := range operators {
		switch operator {
		case "$regex":
			text, isText := value.(string)
			pattern, isPattern := expected.(string)
			if !isText || !isPattern {
				return false
			}
			compiled, err := regexp.Compile(pattern)
			if err != nil || !compiled.MatchString(text) {
				return false
			}
		case "$in":
			list, isList := expected.([]any)
			if !isList {
				return false
			}
			found := false
			for _, item := range list {
				if valuesEqual(value, item) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case "$gte", "$gt", "$lte", "$lt":
			if !compareValues(operator, value, expected) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func valuesEqual(a, b any) bool {
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func compareValues(operator string, value, expected any) bool {
	if left, isNumber := value.(float64); isNumber {
		right, isNumber := expected.(float64)
		if !isNumber {
			return false
		}
		return applyComparison(operator, left, right)
	}
	if left, isText := value.(string); isText {
		right, isText := expected.(string)
		if !isText {
			return false
		}
		return applyComparison(operator, left, right)
	}
	return false
}

func applyComparison[T float64 | string](operator string, left, right T) bool {
	switch operator {
	case "$gte":
		return left >= right
	case "$gt":
		return left > right
	case "$lte":
		return left <= right
	case "$lt":
		return left < right
	}
	return false
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// seededLedger sets up the initial state and returns a context over it.
func seededLedger(t interface{ Fatalf(string, ...any) }) (contractapi.TransactionContextInterface, *mockStub, *TradeContract) {
	ctx, stub := newContext()
	tradeContract := new(TradeContract)
	if _, err := tradeContract.InitLedger(ctx); err != nil {
		t.Fatalf("InitLedger failed: %v", err)
	}
	return ctx, stub, tradeContract
}

// productCodes extracts the codes from a search result for easier comparison.
func productCodes(products []*Product) []string {
	codes := make([]string, 0, len(products))
	for _, product := range products {
		codes = append(codes, product.Code)
	}
	return codes
}

func joined(values []string) string { return strings.Join(values, ",") }
