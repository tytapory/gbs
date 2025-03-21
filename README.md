# GBS

GBS is a lightweight REST API core for P2P transactions, designed to support multiple clients and plugins via built-in user authentication.
The core idea is that user permissions and credentials can be leveraged to easily develop various plugins that seamlessly integrate with the system.


# Getting Started

### 🚀 **Using Docker**
1. **Install Docker** on your system if you haven't already.  
2. Make sure that database settings in config.json **are the same** as in docker-compose.yml.
3. **Run the application with Docker Compose:**  
   ```sh
   docker compose up -d
   ```

### ⚡ **Running Without Docker**
1. **Start PostgreSQL** and ensure that the `config.json` file contains the correct database credentials.  
2. **Run the application manually:**  
   ```sh
   go run cmd/app/main.go
   ```

### 👥 Default Users on First Launch
On the first launch, the system automatically generates passwords for four default users and prints them to the console:
Default users:
- adm            → root user with full privileges  
- fees           → receives all transaction fees  
- money_printer  → has permission to print new money  
- registration   → handles user signups when direct registration is disabled

⚠️ These credentials are generated only once and displayed in the console on first startup.

⚠️ Make sure to **change the passwords immediately** after setup to ensure security.

# 📦 **Getting Started with the API**

After the server is running and default users are created, you can interact with the API using the generated credentials.  
Here’s a simple example of how to authenticate and call a protected endpoint using `curl`:

```sh
# Log in with one of the default users (e.g., adm)
curl -X POST http://localhost:8080/api/v1/login \\
  -H "Content-Type: application/json" \\
  -d '{"username": "adm", "password": "<generated_password>"}'
```

This will return a JSON response with an authentication token:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_expiry": "2025-03-22T12:34:56Z",
  "refresh_token": "abc.def.ghi",
  "refresh_token_expiry": "2025-04-21T12:34:56Z"
}
```

You can then use this token to access other protected endpoints:

```sh
curl -X GET http://localhost:8080/api/v1/getBalances?id=1 \\
  -H "Authorization: Bearer <your_token_here>"
```

This will return the balance data for the user with ID 1.
