#!/usr/bin/env bash
set -euo pipefail

# Happy-path demo: posts a valid order; payment succeeds, inventory is reserved.
ORDER_ID="order-$(date +%s)"
ENDPOINT="${ORDER_SERVICE_URL:-http://localhost:8080}/orders"

echo "POST ${ENDPOINT}"
curl -s -X POST "${ENDPOINT}" \
  -H "Content-Type: application/json" \
  -d "{
    \"order_id\": \"${ORDER_ID}\",
    \"items\": [{\"item_id\": \"widget-1\", \"quantity\": 2}],
    \"payment_amount\": 49.99
  }" | tee /dev/stderr | jq .
