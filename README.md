# GBS
GBS is a lightweight REST API core for P2P transactions, designed to support multiple clients and plugins via built-in user authentication. It enables plugins to independently execute actions on behalf of users, each with its own customizable logic.



# Installation 

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