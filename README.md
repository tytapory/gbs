# GBS
Lightweight core for P2P transactions.

# Installation 

📌 **Installation Guide**  

### 🚀 **Using Docker**
1. **Install Docker** on your system if you haven't already.  
2. **Run the application with Docker Compose:**  
   ```sh
   docker compose up -d
   ```

⚡ **Running Without Docker**
1. **Start PostgreSQL** and ensure that the `config.json` file contains the correct database credentials.  
2. **Run the application manually:**  
   ```sh
   go run cmd/app/main.go
   ```