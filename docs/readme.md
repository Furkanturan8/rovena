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
make up                   # postgres, rabbitmq, identity (+ migrations), nginx gateway
make seed-admin           # creates the first admin from ADMIN_EMAIL / ADMIN_PASSWORD
make test
```

The gateway listens on `http://localhost:8080`. Current API surface (Identity service):

| Method | Path | Access |
| --- | --- | --- |
| POST | `/api/v1/auth/register`, `/login`, `/refresh`, `/logout` | public (rate limited) |
| GET, PATCH | `/api/v1/users/me` | authenticated |
| GET | `/api/v1/users`, `/api/v1/users/:id` | admin |
| PATCH | `/api/v1/users/:id/role` | admin |

Health: `/healthz` (liveness) and `/readyz` (dependencies) on each service port (identity: `8081`).

## Project Goals

Rovena is a portfolio project focused on learning and demonstrating microservices architecture, distributed transactions, event-driven systems, API design, database consistency, automated testing, and production-oriented deployment practices.
