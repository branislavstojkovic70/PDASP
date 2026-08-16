#!/usr/bin/env bash
#
# Adding products to a merchant, one at a time and in bulk.

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_tools; require_identities
suite "04  Products"

ORG=org1; USER_ID=org1admin

merchant="MP${RUN_ID}"
app create-merchant "${merchant}" "Product Shop ${RUN_ID}" SUPERMARKET "PIB${RUN_ID}" 1000 >/dev/null

step "adding one product"
code="PA${RUN_ID}"
created=$(app add-product "${merchant}" "${code}" "Test article" 199.99 25 2027-01-31)
assert_json "${created}" .code "${code}" "the product is stored"
assert_json "${created}" .price "199.99" "the price is recorded"
assert_json "${created}" .quantity "25" "the quantity is recorded"
assert_json "${created}" .expiryDate "2027-01-31" "the expiry date is recorded"

step "the merchant type is denormalized onto the product"
assert_json "${created}" .merchantType "SUPERMARKET" "the product carries the merchant type"

step "the product is listed under its merchant"
assert_contains "$(app merchant-products "${merchant}")" "${code}" "the product is in the merchant offering"

step "a product without an expiry date is allowed"
noexpiry=$(app add-product "${merchant}" "PN${RUN_ID}" "No expiry article" 50 10)
assert_json "${noexpiry}" .expiryDate "" "the expiry date stays empty"

step "adding several products at once"
cat > "${TEST_DIR}/fixtures/.products-${RUN_ID}.json" <<JSON
[
  {"code":"PB1${RUN_ID}","name":"Bulk article one","price":11.5,"quantity":3},
  {"code":"PB2${RUN_ID}","name":"Bulk article two","price":22.5,"quantity":4,"expiryDate":"2028-05-05"}
]
JSON
bulk=$(app add-products "${merchant}" --file "${TEST_DIR}/fixtures/.products-${RUN_ID}.json")
assert_json "${bulk}" 'length' "2" "both products are created in one transaction"
rm -f "${TEST_DIR}/fixtures/.products-${RUN_ID}.json"

step "the merchant now lists four products"
assert_json "$(app merchant "${merchant}")" '.products | length' "4" "the merchant offering has four codes"

step "changing the price"
assert_json "$(app update-price "${code}" 149.49)" .price "149.49" "the new price is stored"

step "restocking"
assert_json "$(app restock "${code}" 15)" .quantity "40" "the quantity grew by fifteen"

summary "04 Products"
