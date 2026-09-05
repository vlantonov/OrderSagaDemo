#!/usr/bin/env bash
set -euo pipefail

# Forced-compensation demo:
# item_id starts with FAIL_ (INVENTORY_FAIL_ITEM_PREFIX default).
# Flow: OrderCreated → PaymentProcessed → ReserveInventory fails →
#       CompensatePayment published → PaymentRefunded → order COMPENSATED.
# Watch traces in Grafana (http://localhost:3000) to see the full rollback span.
ORDER_ID="order-fail-$(date +%s)"
ENDPOINT="${ORDER_SERVICE_URL:-http://localhost:8080}/orders"

echo "POST ${ENDPOINT} (compensation path)"
curl -s -X POST "${ENDPOINT}" \
  -H "Content-Type: application/json" \
  -d "{
    \"order_id\": \"${ORDER_ID}\",
    \"items\": [{\"item_id\": \"FAIL_widget-99\", \"quantity\": 1}],
    \"payment_amount\": 19.99
  }" | tee /dev/stderr | jq .
