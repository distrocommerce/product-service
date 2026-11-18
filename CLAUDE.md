# product-service

Go microservice owning the public product catalog for DistroCommerce.
Shared workflow and service conventions (Go baseline, charts, images, commits, OpenSpec
steps): ../distrocommerce-infra/docs/workflow.md - follow it. This file adds only what is
true for product-service.

## Names and environment
- Repo `product-service`; service name everywhere in the cluster: `product` (Helm release,
  Deployment, Service, image, OTel service.name). Namespace `apps`.
- Database `product` in the compose PostgreSQL 18.6; Secret `product-db` (key DATABASE_URL).
- `make run` listens on :8082 (8080 is kind/Envoy, 8081 is profile-service).
- Public route: api.localtest.me, path prefix /v1/products.

## Rules specific to this service
- The catalog is public: `auth.enabled: false` in the chart. Never read X-User-ID here
  (service-lib strips it on public routes).
- Prices: integer minor units + currency; allowed currencies USD, EUR, UAH; never floats.
- Product IDs are UUIDv7. List pagination is keyset on (created_at, id), newest first.
- Local test data lives in a local-only migration enabled by SEED_LOCAL_DATA=true
  (`make run` and values-local.yaml only); never in staging or production.
- Store test: PRODUCT_TEST_DATABASE_URL (throwaway schema).

## Status
Written by hand to learn the stack; AI-assisted work starts later.