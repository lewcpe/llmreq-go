# **Project Specification: API Key Management Backend**

## **1\. Overview**

This application is a **Go (Golang) backend** designed to act as a middleware/manager for a **LiteLLM** instance. It allows users (authenticated via OAuth2-Proxy) to generate, view, and manage their own API keys for AI model access.  
The application prioritizes fetching data directly from LiteLLM but uses a local SQLite database for historical data that cannot be retrieved from the upstream service (e.g., deleted keys).

## **2\. Tech Stack**

* **Language:** Go 1.23+  
* **Framework:** Echo v4 or Chi (HTTP Router & Middleware)  
* **Database:** SQLite (via mattn/go-sqlite3 driver)  
  * *ORM/Query Builder:* GORM or sqlc (Team preference)  
* **HTTP Client:** Standard net/http  
* **Environment:** Dockerized (Application \+ LiteLLM)  
* **Testing:** Standard testing package (Target: \>75% coverage)  
* **CI/CD:** GitHub Actions

## **3\. Configuration & Environment Variables**

The application must load the following environment variables (using a library like godotenv or viper):

| Variable | Description | Default |
| :---- | :---- | :---- |
| LLMREQ\_PREFIX | Default API prefix | /api |
| LITELLM\_API\_URL | URL of the LiteLLM Docker container | http://litellm:4000 |
| LITELLM\_MASTER\_KEY | The generic master key used to authenticate admin requests to LiteLLM | \- |
| LLMREQ\_DATABASE\_URL | Connection string for SQLite | file:app.db?cache=shared\&mode=rwc |
| LLMREQ\_DEFAULT\_BUDGET | Default lifetime budget cap for standard keys (USD) | 1.0 |
| LLMREQ\_LONGTERM\_KEY\_LIFETIME | Expiration duration for long-term keys (Go duration string, e.g., "9600h") | 9600h (\~400d) |
| LLMREQ\_LONGTERM\_KEY\_LIMIT | Maximum number of active long-term keys allowed per user | 1 |
| LLMREQ\_LONGTERM\_KEY\_BUDGET | Periodic (weekly) budget for long-term keys (USD) | 20 |
| LLMREQ\_MAX\_ACTIVE\_KEY | Maximum total number of active keys allowed per user | 10 |

## **4\. Authentication & User Provisioning**

### **4.1. Trusted Header Authentication**

The app sits behind **OAuth2-Proxy**.

1. **Middleware:** Inspect the HTTP Header X-Forwarded-Email.  
2. **Validation:** If the header is missing, return 401 Unauthorized.  
3. **Normalization:** Convert the email to lowercase. This value is referred to as current\_user\_id.

### **4.2. JIT User Provisioning (LiteLLM Sync)**

On every authenticated request (middleware logic):

1. Check if current\_user\_id exists in LiteLLM using GET /user/info.  
2. **If User does NOT exist:**  
   * Call LiteLLM POST /user/new to create the user.  
   * Set user\_id \= lower(email).  
   * Set user\_email \= lower(email).  
   * Set default budget settings if applicable.

## **5\. Data Model & Storage Strategy**

### **5.1. Source of Truth: LiteLLM**

* **Active Keys:** Stored in LiteLLM. Fetched in real-time.  
* **User Budget:** Stored in LiteLLM. Fetched in real-time.

### **5.2. Local Storage: SQLite**

Used *only* to satisfy the requirement of showing **Previous Keys** (revoked/deleted keys) and enforcing limits on key types.  
**Table: key\_history**

* id: Integer, PK, Auto-increment  
* user\_id: String (Email, Indexed)  
* litellm\_key\_id: String (The unique ID/prefix from LiteLLM)  
* key\_name: String (User provided alias)  
* key\_mask: String (e.g., sk-...1234)  
* key\_type: String (standard or long-term)  
* created\_at: Datetime  
* revoked\_at: Datetime (Nullable)  
* status: String (active, revoked)

## **6\. API Endpoints**

**Base Path:** /api

### **6.1. User Dashboard**

**GET /api/me**

* **Logic:**  
  * Call LiteLLM GET /user/info/{user\_id}.  
* **Response (JSON):**  
  * user\_id (email)  
  * max\_budget  
  * spend (Current total spend).

### **6.2. Key Management**

**GET /api/keys/active**

* **Logic:**  
  * Call LiteLLM GET /key/list (filtered by user\_id).  
  * Filter response to exclude expired/invalid keys if LiteLLM returns them.  
  * Sync/Update the local key\_history table if any discrepancies are found.  
* **Response:** List of active key objects (mask, name, created\_at, spend, type).

**GET /api/keys/history**

* **Logic:**  
  * Query local SQLite key\_history table where user\_id matches and (status is 'revoked' OR revoked\_at is not null).  
* **Response:** List of historical keys.

**POST /api/keys**

* **Body:**  
  {  
    "name": "my-project-key",  
    "budget": 1.5, // budget could be omitted or must be less than LLMREQ\_DEFAULT\_BUDGET or LLMREQ\_LONGTERM\_KEY\_BUDGET depends on key type  
    "type": "standard" // or "long-term"  
  }

* **Logic:**  
  1. **Check Limits:**  
     * **Global Limit:** Count total active keys. If \>= LLMREQ\_MAX\_ACTIVE\_KEY, reject.  
     * **Long-term Limit:** If type is long-term, count existing long-term keys. If \>= LLMREQ\_LONGTERM\_KEY\_LIMIT, reject.  
  2. **Call LiteLLM:**  
     * Call POST /key/generate.  
     * Payload: { "user\_id": current\_user\_id, "key\_alias": name, "max\_budget": ..., "duration": ... }  
  3. **Persist Metadata:**  
     * Save metadata to local SQLite key\_history.  
* **Response:** The full raw API key.

**DELETE /api/keys/{key\_id}**

* **Logic:**  
  1. Call LiteLLM POST /key/delete.  
  2. Update local SQLite key\_history: set status \= revoked.  
* **Response:** 200 OK.

## **7\. Business Logic Details**

### **7.1. Budget Display**

The dashboard requires "Total used budget".

* **Implementation:**  
  * Check the LiteLLM User Info object.  
  * **Fallback:** Return the spend attribute from the User object (Total Life Time Spend) and label it clearly in the UI.

### **7.2. Error Handling**

* If LiteLLM is down: Return HTTP 503 Service Unavailable.  
* If User is unauthorized: Return HTTP 401 Unauthorized.  
* If limits are exceeded: Return HTTP 400 Bad Request.

## **8\. Development & Infrastructure**

### **8.1. LiteLLM Configuration**

* **Models:** A litellm\_config.yaml must be provided.  
* **Mockup Models:** For testing and development, include a model named fake-gpt-test that uses a mock provider to allow budget increment testing without real costs.

### **8.2. Testing Strategy**

* **Tool:** go test  
* **Coverage Requirement:** Minimum 75% code coverage.  
* **Integration Tests:**  
  * Spin up the LiteLLM container with the fake-gpt-test model.  
  * Generate a key via the App API.  
  * Use the generated key to call the LiteLLM fake-gpt-test endpoint.  
  * Verify spend increases in the App Dashboard (GET /api/me).

### **8.3. CI/CD Pipeline**

* **Platform:** GitHub Actions.  
* **Triggers:**  
  * **Pull Requests:**  
    * Linting (golangci-lint).  
    * Formatting (gofmt check).  
    * Tests (go test ./... \-cover).  
  * **Main Push (Merge):**  
    * Build Docker Image (docker build \-t ...).  
    * Login to Container Registry (GHCR or Docker Hub).  
    * Push Docker Image.  
* **Workflow Steps:**  
  1. Checkout Code.  
  2. Set up Go environment.  
  3. Spin up LiteLLM Service container (using docker compose or service definition).  
  4. Run Tests.  
  5. Check Coverage Report (fail if \< 75%).  
  6. **(On Main Only)** Build and Push Docker Image.