#!/usr/bin/env bash
#
# Registering merchants and the merchant type catalogue.

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_tools; require_identities
suite "02  Merchant types and merchants"

ORG=org1; USER_ID=org1admin

step "the catalogue is on the ledger, not hard coded in the application"
types=$(app merchant-types)
assert_json "${types}" 'length >= 5' "true" "the catalogue holds at least five types"
assert_contains "${types}" "SUPERMARKET" "SUPERMARKET is in the catalogue"

step "adding a new type"
type_code="TYPE${RUN_ID}"
created=$(app create-merchant-type "${type_code}" "Test line of business" "created by test-02")
assert_json "${created}" .code "${type_code}" "the new type is stored"
assert_json "$(app merchant-type "${type_code}")" .name "Test line of business" "the new type reads back"

step "registering a merchant"
merchant="MT${RUN_ID}"
result=$(app create-merchant "${merchant}" "Test Shop ${RUN_ID}" "${type_code}" "PIB${RUN_ID}" 5000)
assert_json "${result}" .merchantId "${merchant}" "the merchant is stored"
assert_json "${result}" .balance "5000" "the opening balance is recorded"
assert_json "${result}" '.products | length' "0" "a new merchant has no products"

step "the merchant appears in the listing"
assert_contains "$(app merchants)" "${merchant}" "the merchant is in the full listing"

step "registering several merchants at once"
cat > "${TEST_DIR}/fixtures/.merchants-${RUN_ID}.json" <<JSON
[
  {"merchantId":"MB1${RUN_ID}","name":"Bulk One","type":"${type_code}","taxId":"111${RUN_ID}","openingBalance":100},
  {"merchantId":"MB2${RUN_ID}","name":"Bulk Two","type":"${type_code}","taxId":"222${RUN_ID}","openingBalance":200}
]
JSON
bulk=$(app create-merchants --file "${TEST_DIR}/fixtures/.merchants-${RUN_ID}.json")
assert_json "${bulk}" 'length' "2" "both merchants are created in one transaction"
rm -f "${TEST_DIR}/fixtures/.merchants-${RUN_ID}.json"

step "changing a merchant type propagates to its products"
app add-product "${merchant}" "PP${RUN_ID}" "Propagation probe" 100 5 >/dev/null
app change-merchant-type "${merchant}" SUPERMARKET >/dev/null
assert_json "$(app product "PP${RUN_ID}")" .merchantType "SUPERMARKET" \
  "the product picked up the new merchant type"

summary "02 Merchants"
