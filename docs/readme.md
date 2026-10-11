# Rovena

Rovena is a modern fashion and accessories e-commerce marketplace built with **Go, React, and TypeScript**. The project is designed to demonstrate production-oriented software engineering practices through a microservices architecture, reliable order processing, and scalable application design.

## Architecture

Rovena follows a domain-oriented microservices architecture with independently organized services, database ownership boundaries, synchronous REST APIs, and asynchronous event-driven communication.

### Services

* **Identity Service:** Authentication, user accounts, and role-based access control.
* **Catalog Service:** Products, categories, product variants, and product information.
* **Inventory Service:** Stock management, reservations, and inventory movements.
* **Order Service:** Shopping carts, checkout orchestration, and order lifecycle management.
* **Payment Service:** Provider-independent payment abstraction and simulated payment processing.

### Tech Stack

* **Backend:** Go (Golang)
* **Frontend:** React, TypeScript, Vite
* **API:** REST / HTTP
* **Database:** PostgreSQL
* **Messaging:** RabbitMQ
* **Infrastructure:** Docker, Docker Compose, Nginx
* **Observability:** OpenTelemetry
* **Payments:** Mock payment provider; no real payment processing

### Engineering Principles

* Clear service boundaries and database ownership
* Modular, testable, and maintainable code
* Event-driven communication for asynchronous workflows
* Transactional Outbox and idempotent message processing
* Saga-based order and payment orchestration
* Secure authentication and authorization
* Automated testing and CI/CD

## Project Structure

```text
rovena/
├── services/
│   ├── identity/
│   ├── catalog/
│   ├── inventory/
│   ├── order/
│   └── payment/
├── frontend/
├── infrastructure/
├── docs/
│   └── architecture.md
├── docker-compose.yml
└── README.md
```

Each service owns its business logic, data access, migrations, and API contracts.

## Getting Started

The development environment runs locally with Docker Compose.

```bash
cp .env.example .env      # fill in every value; JWT_SECRET: openssl rand -base64 48
make up                   # postgres, rabbitmq, identity, catalog, inventory and payment (+ migrations), nginx gateway
make seed-admin           # creates the first admin from ADMIN_EMAIL / ADMIN_PASSWORD
make seed-catalog         # loads demo categories and products
make seed-inventory       # gives every demo variant an opening stock
make test
```

The gateway listens on `http://localhost:8080`. Current API surface:

**Identity**

| Method | Path | Access |
| --- | --- | --- |
| POST | `/api/v1/auth/register`, `/login`, `/refresh`, `/logout` | public (rate limited) |
| GET, PATCH | `/api/v1/users/me` | authenticated |
| GET | `/api/v1/users`, `/api/v1/users/:id` | admin |
| PATCH | `/api/v1/users/:id/role` | admin |

**Catalog**

| Method | Path | Access |
| --- | --- | --- |
| GET | `/api/v1/categories`, `/categories/:slug` | public |
| GET | `/api/v1/products` (`q`, `category`, `size`, `color`, `min_price`, `max_price`, `sort`, `page`) | public |
| GET | `/api/v1/products/:idOrSlug` | public |
| GET, POST, PATCH, DELETE | `/api/v1/admin/products[/:id]`, `/admin/products/:id/variants[/:variantId]`, `/admin/categories[/:id]` | admin |

**Inventory**

| Method | Path | Access |
| --- | --- | --- |
| GET | `/api/v1/stock?variant_ids=a,b,c` | public |
| GET, POST | `/api/v1/admin/stock[/:variantId]` | admin |
| POST | `/api/v1/admin/stock/:variantId/receipts`, `/adjustments` | admin |
| GET | `/api/v1/admin/stock/:variantId/movements` | admin |
| POST, GET | `/api/v1/internal/reservations[/:id]`, `/:id/commit`, `/:id/release`, `/by-order/:orderId` | internal key, not routed by the gateway |

Reservations are the service-to-service API the Order Service will drive: reserving is idempotent per
order, committing turns a hold into a sale, releasing is the compensating action, and holds that are
never resolved expire after `RESERVATION_TTL` (default 15 minutes).

OpenAPI schemas for every service live in [`docs/openapi/`](openapi/README.md) (import them into Postman/Insomnia/Bruno to try the API).

**Payment**

Payments are created only by `PaymentRequested` events (RabbitMQ); the HTTP API reads them. There is no endpoint
that creates or changes a payment.

| Method | Path | Access |
| --- | --- | --- |
| GET | `/api/v1/payments/by-order/:orderId` | signed-in user, own payments only |
| GET | `/api/v1/admin/payments[/:id]` (`status`, `order_id`, `page`, `page_size`) | admin |
| GET | `/api/v1/internal/payments/by-order/:orderId` | internal key, not routed by the gateway |

The provider is a fake whose answer is chosen by the test card token (`tok_visa`, `tok_declined`, `tok_flaky`, ...);
see [`docs/openapi/payment.yaml`](openapi/payment.yaml) for the events and the full list.

Health: `/healthz` (liveness) and `/readyz` (dependencies) on each service port (identity: `8081`, catalog: `8082`, inventory: `8083`, payment: `8085`).

## Project Goals

Rovena is a portfolio project focused on learning and demonstrating microservices architecture, distributed transactions, event-driven systems, API design, database consistency, automated testing, and production-oriented deployment practices.
