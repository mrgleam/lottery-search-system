# Lottery Search System

A high-performance, concurrent search and reservation engine for 6-digit lottery tickets (up to 10,000,000 tickets across 1,000,000 unique numbers). The system matches wildcard patterns arithmetically and dispenses tickets to concurrent users without double-selling or lock contention.

---

## 📚 Documentation Index

This repository contains both a complete runnable Go implementation and a 142-step training curriculum:

| Document | Audience | Description |
|---|---|---|
| [**`lottery-training-course.md`**](lottery-training-course.md) | Students / Engineers | Complete 142-step training course across 12 modules (0–11), taking you from basic programming concepts to a scaled production service. |
| [**`lottery-instructor-manual.md`**](lottery-instructor-manual.md) | Instructors / Mentors | Guide for teaching the course, physical exercises, setup requirements, conceptual background, and student troubleshooting tips. |
| [**`lottery-go/README.md`**](lottery-go/README.md) | Developers | Technical reference for the Go implementation, architecture, API documentation, testing, and operational scaling guidelines. |
| [**`lottery-go/STATE-SYNC.md`**](lottery-go/STATE-SYNC.md) | Developers / Architects | Deep dive into how in-memory availability hints synchronize with PostgreSQL safely (over-reporting vs. under-reporting rules). |

---

## 🚀 Quick Start

The reference implementation is located in the [`lottery-go/`](lottery-go/) directory.

### Prerequisites
- Go 1.22+
- Docker & Docker Compose (for PostgreSQL)
- Make

### Running the Service

```bash
cd lottery-go

# 1. Download dependencies
make deps

# 2. Run unit tests
make test

# 3. Start PostgreSQL container
make db

# 4. Run database integration tests
make itest

# 5. Seed test data (100,000 tickets)
make seed-small

# 6. Start the API server on :8080
make run
```

### Example Usage

#### 1. Search and Reserve Tickets
Search tickets matching a wildcard pattern (e.g., ending in `23`) and place a lease hold:
```bash
curl -s -X POST localhost:8080/search \
  -H "Content-Type: application/json" \
  -d '{"pattern":"****23","count":5,"holder":"alice"}'
```

#### 2. Confirm a Reservation
Confirm the purchase of reserved ticket #42 before the lease expires:
```bash
curl -s -X POST localhost:8080/reservations/42/confirm \
  -H "Content-Type: application/json" \
  -d '{"holder":"alice"}'
```

#### 3. Release a Reservation
Release a held ticket back to circulation:
```bash
curl -s -X POST localhost:8080/reservations/42/release \
  -H "Content-Type: application/json" \
  -d '{"holder":"alice"}'
```

---

## 🏛️ Architecture & Key Concepts

1. **Arithmetic Pattern Matching**  
   A 6-digit number has exactly 1,000,000 possible values (`000000` to `999999`). Wildcard expansion (`Pattern.Candidates`) computes matching candidate numbers mathematically rather than scanning storage.
   
2. **Atomic Non-Blocking Claims (`SKIP LOCKED`)**  
   Tickets are locked, verified, and claimed in a single atomic SQL statement using PostgreSQL's `FOR UPDATE SKIP LOCKED`. Concurrent workers step over locked rows rather than blocking each other.

3. **Three Backend Implementations (`lottery.TicketStore`)**  
   - **`MemoryStore`** (`store.go`): In-memory reference implementation using CSR indexes and bitmaps.
   - **`postgres.Store`** (`postgres/store.go`): Direct PostgreSQL backend using index scans and atomic queries.
   - **`postgres.HybridStore`** (`postgres/hybrid.go`): High-throughput production store combining a 4MB in-memory availability hint with PostgreSQL consistency.

4. **Leases vs Database Locks**  
   Database row locks exist only for milliseconds during the claim statement. The reservation hold (e.g., 2 minutes for checkout) is stored as a timestamp (`lease_until`) in the database.

---

## 📂 Repository Structure

```
.
├── README.md                      # Root documentation index (this file)
├── lottery-training-course.md     # 142-step training curriculum (Modules 0-11)
├── lottery-instructor-manual.md   # Instructor guide and teaching manual
└── lottery-go/                    # Go implementation
    ├── README.md                  # Detailed Go package documentation
    ├── STATE-SYNC.md              # In-memory hint and PostgreSQL sync guide
    ├── Makefile                   # Build, test, run, and benchmark tasks
    ├── docker-compose.yml         # Local PostgreSQL configuration
    ├── pattern.go                 # Pattern parsing and odometer arithmetic
    ├── candidates.go              # Scrambled candidate number generation
    ├── walker.go                  # Contention-spreading permutations
    ├── index.go / bitmap.go       # In-memory CSR index and 64-bit bitmap
    ├── ticketstore.go             # Domain interfaces and errors
    ├── store.go                   # In-memory TicketStore
    ├── server.go                  # HTTP REST API handlers
    ├── cmd/
    │   ├── api/                   # API service entrypoint
    │   ├── seed/                  # High-speed bulk data seeder (PostgreSQL COPY)
    │   └── loadgen/               # Concurrency and load testing generator
    ├── postgres/                  # PostgreSQL backends and hybrid store
    └── storetest/                 # Shared conformance test suite
```

---

## 🎓 Training Course Outline

The included [training course](lottery-training-course.md) is divided into 12 self-contained modules:

- **Module 0:** Environment & Tooling Setup (Go, Docker, Make)
- **Module 1:** Go Foundations for the System
- **Module 2:** The Problem Space & Pattern Arithmetic (No code)
- **Module 3:** Pattern Matching & Odometer (TDD)
- **Module 4:** CSR Index & Bitmap Structure
- **Module 5:** In-Memory Ticket Store Implementation
- **Module 6:** Concurrency, Leases, and State Machines
- **Module 7:** REST API & HTTP Layer
- **Module 8:** PostgreSQL Backend & Atomic Claims (`SKIP LOCKED`)
- **Module 9:** Hybrid Store, Availability Hints, & PostgreSQL Notifications
- **Module 10:** Production Service Packaging & Deployment
- **Module 11:** Performance Benchmarking, Load Sweeps, & Scaling

---

## 🧪 Testing

Run the full test suite from the `lottery-go` folder:

```bash
cd lottery-go

# Unit tests (in-memory, no external dependencies)
make test

# Integration tests (requires Docker/Postgres)
make itest

# Race detector
go test -race ./...
```
