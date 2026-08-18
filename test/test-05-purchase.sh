#!/usr/bin/env bash
#
# Buying a product: the central piece of business logic.

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_tools; require_identities
suite "05  Purchase"

ORG=org1; USER_ID=org1admin
merchant="MS${RUN_ID}"
product="PS${RUN_ID}"
app create-merchant "${merchant}" "Sale Shop ${RUN_ID}" SUPERMARKET "PIB${RUN_ID}" 0 >/dev/null
app add-product "${merchant}" "${product}" "Sale article" 100 10 >/dev/null

ORG=org2; USER_ID=org2user1
customer="CS${RUN_ID}"
app create-customer "${customer}" "Sale" "Buyer" "sale${RUN_ID}@example.com" 1000 >/dev/null

step "buying three units"
result=$(app buy "${customer}" "${merchant}" "${product}" 3)
assert_json "${result}" .invoice.total "300" "the total is price times quantity"
assert_json "${result}" .customerBalance "700" "the customer was charged"
assert_json "${result}" .merchantBalance "300" "the merchant was credited"
assert_json "${result}" .remainingQuantity "7" "the stock went down by three"
assert_json "${result}" .productRemoved "false" "the product is still on offer"

invoice_id=$(echo "${result}" | jq -r .invoice.id)

step "the invoice is linked to both parties"
assert_contains "$(app customer "${customer}")" "${invoice_id}" "the invoice is on the customer"
assert_contains "$(app merchant "${merchant}")" "${invoice_id}" "the invoice is on the merchant"

step "the invoice reads back with a snapshot of the product"
invoice=$(app invoice "${invoice_id}")
assert_json "${invoice}" .productName "Sale article" "the invoice carries the product name"
assert_json "${invoice}" .unitPrice "100" "the invoice carries the unit price"
assert_not_empty "$(echo "${invoice}" | jq -r .date)" "the invoice carries a date"

step "the invoice appears in both listings"
assert_contains "$(app invoices --customer "${customer}")" "${invoice_id}" "listed under the customer"
assert_contains "$(app invoices --merchant "${merchant}")" "${invoice_id}" "listed under the merchant"

step "filtering invoices by amount"
assert_json "$(app invoices --customer "${customer}" --min 250)" 'length' "1" "the invoice is above 250"
assert_json "$(app invoices --customer "${customer}" --min 500)" 'length' "0" "the invoice is below 500"

step "buying the remaining stock removes the product"
result=$(app buy "${customer}" "${merchant}" "${product}" 7)
assert_json "${result}" .productRemoved "true" "the product is flagged as removed"
assert_json "${result}" .remainingQuantity "0" "no stock is left"

message=$(app_expect_failure product "${product}")
assert_contains "${message}" "does not exist" "the product is gone from the world state"
assert_json "$(app merchant "${merchant}")" '.products | index("'"${product}"'") // "absent"' "absent" \
  "the code is gone from the merchant offering"

step "the invoice survives the product being deleted"
assert_json "$(app invoice "${invoice_id}")" .productName "Sale article" \
  "the old invoice still names the product"

summary "05 Purchase"
