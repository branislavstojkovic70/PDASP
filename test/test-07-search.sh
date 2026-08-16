#!/usr/bin/env bash
#
# Product searches, including the combined query that is the whole reason the
# assignment mandates CouchDB.

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_tools; require_identities
suite "07  Searches (CouchDB rich queries)"

# Searching is a read only operation, so it runs as the Org3 "Regulator" identity.
ORG=org3; USER_ID=org3user1

step "by name, case insensitive"
by_name=$(app search-name MILK)
assert_contains "${by_name}" "P001" "uppercase MILK finds 'Milk 2.8% 1L'"
assert_contains "${by_name}" "P010" "and also 'Baby milk formula 400g'"

step "by code"
assert_contains "$(app search-code P00)" "P001" "a code prefix matches"

step "by merchant type"
by_type=$(app search-type AUTO_PARTS)
assert_json "${by_type}" 'length >= 3' "true" "the auto parts merchant has at least three products"
assert_json "${by_type}" 'all(.merchantType == "AUTO_PARTS")' "true" "every hit is of that type"

step "by price range"
by_price=$(app search-price 100 400)
assert_json "${by_price}" 'all(.price >= 100 and .price <= 400)' "true" "every hit is inside the range"
assert_json "${by_price}" 'length >= 3' "true" "the range returns several products"

step "combined: name, merchant type set and price range in one selector"
combined=$(app search --name milk --type SUPERMARKET,PHARMACY --price-from 1000 --price-to 2000 --sort asc)
assert_json "${combined}" 'length' "1" "only the pharmacy product survives all three filters"
assert_json "${combined}" '.[0].code' "P010" "and it is P010"

step "combined without the price bound returns both"
both=$(app search --name milk --type SUPERMARKET,PHARMACY --sort asc)
assert_json "${both}" 'length' "2" "dropping the price filter widens the result"
assert_json "${both}" '.[0].price <= .[1].price' "true" "the result is sorted by price ascending"

step "descending sort"
desc=$(app search --sort desc)
assert_json "${desc}" '.[0].price >= .[-1].price' "true" "the first hit is not cheaper than the last"

step "expiry date range, excluding products without an date"
expiring=$(app expiring 2026-10-01)
assert_json "${expiring}" 'all(.expiryDate != "")' "true" "products without an expiry date are excluded"
assert_json "${expiring}" 'all(.expiryDate <= "2026-10-01")' "true" "every hit expires before the date"
assert_json "${expiring}" 'all(.quantity > 0)' "true" "every hit is still in stock"

step "products of one merchant"
assert_json "$(app merchant-products M002)" 'all(.merchantId == "M002")' "true" "every hit belongs to M002"

step "pagination"
page1=$(app search-paged --page-size 4)
assert_json "${page1}" .fetchedCount "4" "the first page holds four records"
bookmark=$(echo "${page1}" | jq -r .nextBookmark)
assert_not_empty "${bookmark}" "the first page returns a bookmark"

page2=$(app search-paged --page-size 4 --bookmark "${bookmark}")
first_page_codes=$(echo "${page1}" | jq -r '[.products[].code] | join(",")')
second_page_codes=$(echo "${page2}" | jq -r '[.products[].code] | join(",")')
assert_ne "${first_page_codes}" "${second_page_codes}" "the second page differs from the first"

step "a search with no criteria returns everything"
assert_json "$(app search)" 'length >= 13' "true" "an empty selector returns the whole catalogue"

summary "07 Searches"
