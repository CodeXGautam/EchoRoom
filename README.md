# EchoRoom

## Distributed Real-Time Communication Platform

EchoRoom is a cloud-native, distributed real-time communication platform engineered to deliver scalable, low-latency messaging experiences. The platform leverages WebSocket-based communication, Redis Pub/Sub messaging infrastructure, PostgreSQL persistence, and Kubernetes orchestration to provide a robust and horizontally scalable chat ecosystem.

The project is designed around modern backend engineering principles, emphasizing distributed systems, event-driven architectures, observability, fault tolerance, and cloud-native deployment strategies.

---

## Key Features

### Real-Time Messaging

* Native WebSocket communication
* Low-latency message delivery
* Persistent message storage
* Channel-based conversations
* Private messaging support

### Authentication & Security

* JWT-based authentication
* Secure password hashing using bcrypt
* Protected API endpoints
* Session management

### Distributed Architecture

* Redis Pub/Sub synchronization
* Multi-instance message broadcasting
* Stateless backend services
* Horizontal scalability

### Cloud-Native Infrastructure

* Containerized services
* Kubernetes deployment support
* Load balancing and service discovery
* Graceful shutdown handling
* Health monitoring endpoints

### Observability

* Structured logging using slog
* Centralized log aggregation support
* Metrics collection readiness
* Distributed tracing integration support

---

## Technology Stack

### Backend

| Technology        | Purpose                 |
| ----------------- | ----------------------- |
| Go                | Backend Development     |
| Gin               | HTTP Framework          |
| Gorilla WebSocket | Real-Time Communication |
| Ent ORM           | Database Access Layer   |
| PostgreSQL        | Primary Database        |
| Redis             | Pub/Sub Messaging       |
| JWT               | Authentication          |
| slog              | Structured Logging      |

### Frontend

| Technology   | Purpose            |
| ------------ | ------------------ |
| Next.js      | Frontend Framework |
| React        | User Interface     |
| TypeScript   | Type Safety        |
| Tailwind CSS | Styling            |
| Zustand      | State Management   |

### Infrastructure

| Technology     | Purpose           |
| -------------- | ----------------- |
| Docker         | Containerization  |
| Docker Compose | Local Development |
| Kubernetes     | Orchestration     |
| NGINX Ingress  | Traffic Routing   |

---

## System Architecture

```text
                    ┌─────────────────┐
                    │   Next.js UI    │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │   Gin API Layer │
                    └────────┬────────┘
                             │
      ┌──────────────────────┼──────────────────────┐
      │                      │                      │
      ▼                      ▼                      ▼

┌────────────┐       ┌──────────────┐      ┌──────────────┐
│ Auth Layer │       │ WebSocket Hub│      │ Chat Service │
└─────┬──────┘       └──────┬───────┘      └──────┬───────┘
      │                     │                     │
      └─────────────┬───────┴───────┬─────────────┘
                    ▼               ▼

              ┌─────────┐     ┌────────────┐
              │ Redis   │     │ PostgreSQL │
              └─────────┘     └────────────┘
```

---

## Project Structure

```text
echoroom/
│
├── frontend/
│   ├── app/
│   ├── components/
│   ├── hooks/
│   ├── services/
│   └── store/
│
├── backend/
│   ├── cmd/
│   ├── internal/
│   │   ├── auth/
│   │   ├── chat/
│   │   ├── ws/
│   │   ├── redis/
│   │   ├── db/
│   │   ├── middleware/
│   │   ├── config/
│   │   └── logger/
│   │
│   ├── ent/
│   │   └── schema/
│   │
│   └── pkg/
│
├── infra/
│   ├── docker/
│   ├── k8s/
│   └── helm/
│
└── README.md
```

---

## Installation Requirements

### Backend Environment

* Go 1.24+
* PostgreSQL 16+
* Redis 7+
* Docker Engine
* Docker Compose

### Frontend Environment

* Node.js 22+
* npm 10+

### Infrastructure Environment

* Docker
* Kubernetes Cluster
* kubectl
* Helm

---

## Environment Configuration

### Backend Variables

```env
PORT=8080

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=echoroom

REDIS_ADDR=localhost:6379

JWT_SECRET=your-secret-key
```

---

## Core Data Model

### User

* Unique User ID
* Username
* Email Address
* Password Hash
* Creation Timestamp

### Room

* Room Identifier
* Room Name
* Room Type
* Creator Information
* Creation Timestamp

### Message

* Message Identifier
* Room Reference
* Sender Reference
* Message Content
* Timestamp

---

## Engineering Principles

EchoRoom follows a set of engineering principles commonly adopted in production-grade distributed systems:

* Separation of Concerns
* Stateless Service Design
* Event-Driven Communication
* Structured Logging
* Dependency Injection
* Horizontal Scalability
* Graceful Failure Recovery
* Cloud-Native Deployment Practices

---

## Future Enhancements

* Presence Tracking
* Read Receipts
* Media Attachments
* Voice Channels
* End-to-End Encryption
* Distributed Tracing
* Prometheus Monitoring
* Grafana Dashboards

---

## Developer

**Himanshu Sharma**

