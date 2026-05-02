# Post-MVP Roadmap: Towards Production-Ready

This document outlines the prioritized enhancements and hardening steps required to transition `kong-cert-lite` from a functional MVP to a reliable, production-ready internal tool.

## Phase 1: Security and Hardening (Critical)

### 1. Operator Login
Current status: Implemented with an optional built-in login page and signed session cookie.
- **Goal**: Protect the UI and API from unauthorized access.
- **Plan**: Implement a simple login flow using environment variables (`APP_USERNAME`, `APP_PASSWORD`).
- **Benefit**: Ensures only authorized operators can manage certificates and sync to Kong.

### 2. Secret Hardening
- **Goal**: Move beyond simple environment variables for sensitive data.
- **Plan**: Update Docker configuration to support Docker Secrets or specific secret volumes.
- **Benefit**: Reduces the risk of secret exposure in process listings or logs.

## Phase 2: Reliability and Observability (High)

### 3. Failure Notifications (Slack/Discord Webhook)
- **Goal**: Immediate awareness of job failures.
- **Plan**: Add a notification service that sends messages to a configured webhook when an ACME issue/renew or Kong sync job fails.
- **Benefit**: Prevents silent failures of the auto-renewal scheduler.

### 4. Backup & Restore Validation
- **Goal**: Guarantee data durability.
- **Plan**:
    - Create a validated recovery script.
    - Document the "Point-in-Time" recovery process for the SQLite database and `/data` volume.
- **Benefit**: Minimizes downtime during infrastructure migrations or failures.

## Phase 3: User Experience and Visibility (Medium)

### 5. Expiry Dashboard
- **Goal**: High-level overview of certificate health.
- **Plan**: Create a dashboard view summarizing:
    - Days remaining for all active certificates.
    - Count of certificates in `warning` or `failed` states.
    - Last 5 failed jobs across the system.
- **Benefit**: Provides proactive monitoring of the entire certificate fleet.

### 6. Enhanced Job Logs
- **Goal**: Easier debugging.
- **Plan**: Add filtering and search capabilities to the job log UI.
- **Benefit**: Saves time when investigating historical failures.

## Phase 4: Flexibility and Scale (Low)

### 7. Multi-DNS Provider Support
- **Goal**: Support environments outside of Cloudflare.
- **Plan**: Refactor the ACME adapter to dynamically load different `lego` DNS providers based on configuration.
- **Benefit**: Allows the tool to be used across diverse cloud environments (AWS, GCP, Azure, etc.).

### 8. Audit Logging
- **Goal**: Compliance and tracking.
- **Plan**: Implement a read-only audit log tracking "Who changed what and when."
- **Benefit**: Important for larger teams or regulated environments.
