<div align="center">
  <h1>NexusAPI Flow 🌊</h1>
  <p><b>Unified LLM API Key Management and Smart Proxy Gateway</b></p>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go">
  <img alt="Gin" src="https://img.shields.io/badge/Gin-Framework-0088cc?style=flat-square">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-Supported-2496ED?style=flat-square&logo=docker">
  <img alt="License" src="https://img.shields.io/badge/License-GPLv3-blue.svg?style=flat-square">
</div>

## 📖 Overview

**NexusAPI Flow** is an advanced API Key management platform and smart proxy redistributor designed for Large Language Models (LLMs). It seamlessly unifies multiple upstream LLM providers (OpenAI, Anthropic, Gemini, etc.) behind a single standard `/v1/chat/completions` endpoint. 

Whether you're managing keys for a team, distributing tokens to users, or building a robust failover gateway for production, NexusAPI offers a powerful, elegant, and lightning-fast solution.

## ✨ Core Features

* **🔌 Unified Proxy Gateway**: Transforms various backend models into a standard OpenAI-compatible API format (with full SSE streaming support).
* **🧠 Smart Routing**:
  * **429 Fallback**: Automatically switches to the next available key if the current one hits a rate limit.
  * **Context Scaling**: Automatically upgrades to models with larger context windows if token length is exceeded.
* **👥 Multi-Tenant Architecture**: Complete user management system with Admin and Regular User roles.
* **🔐 Robust API Key Ecosystem**:
  * **Global Pool**: Upstream keys managed by Admins, accessible via generated NexusKeys.
  * **Private Pool**: Users can bring and manage their own upstream keys.
* **📊 Granular Usage Quotas**: Track and limit usage per user by Total Lifetime, Monthly, Weekly, and rolling 5-Hour windows.
* **🛡️ Security First**: Includes built-in JWT authentication, Bcrypt password hashing, and 2FA (TOTP) support.
* **🌍 True Localization (i18n)**: Out-of-the-box support for 7 languages (EN, ZH, JA, RU, KO, FR, ES) with a sleek, responsive Glassmorphism UI.
* **🎁 Easter Egg**: Built-in free interactive model endpoint.

---

## 🚀 Quick Start & Remote Deployment

NexusAPI Flow is designed for effortless remote server deployment via Docker Compose.

### Prerequisites
* A Linux server (Ubuntu/Debian/CentOS etc.)
* [Docker](https://docs.docker.com/get-docker/) & [Docker Compose](https://docs.docker.com/compose/install/) installed
* Git

### Step-by-Step Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/your-repo/Nexus-API.git
   cd Nexus-API
   ```

2. **Run the Interactive Setup Script:**
   We provide a fully interactive bash script to handle configuration, environment building, and Docker orchestration.
   ```bash
   chmod +x install.sh
   ./install.sh
   ```

3. **Follow the Prompts:**
   The script will ask you for a few basic settings (press `Enter` to use default values):
   * **Admin Username**: Define your root administrator username (default: `nexusapi`).
   * **Admin Password**: Define your root administrator password (default: `nexusapi`).
   * **Panel Port**: The port exposed to the host for web access (default: `8080`). If your port `8080` is in use, switch it to `8085` or another available port.

4. **Enjoy!**
   The script will automatically compile the Go binary inside a multi-stage Dockerfile and launch the container.
   Access your brand new dashboard at: `http://<your-server-ip>:<Port>` (e.g. `http://localhost:8080`).

---

## 🌐 Advanced Configuration (Nginx Reverse Proxy)

If you are exposing NexusAPI Flow to the public internet, it is highly recommended to use **Nginx** as a reverse proxy for SSL/TLS termination. 

Our dynamic `BASE_PATH` architecture allows you to mount NexusAPI on the root domain (`/`) or any sub-path (e.g., `/nexus/`) seamlessly without rebuilding.

**Example Nginx Configuration for a sub-path (`/nexus/`)**:
```nginx
server {
    listen 80;
    server_name api.yourdomain.com;

    location /nexus/ {
        proxy_pass http://127.0.0.1:8080/; # Change 8080 to your Panel Port
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Critical for SSE streaming support
        proxy_set_header Connection '';
        proxy_http_version 1.1;
        chunked_transfer_encoding off;
        proxy_buffering off;
        proxy_cache off;
    }
}
```

---

## 🔗 Connecting to Local/Internal APIs (e.g. Ollama)

If you are running an AI model locally on the same host server (such as Ollama or vLLM), **do not** use `localhost` or `127.0.0.1` as the Base URL, as this will point to the inside of the Docker container itself. 

Instead, NexusAPI is pre-configured with a network bridge. Use the following special hostname as your Base URL to route traffic out to the host machine:
* `http://host.docker.internal:<PORT>`

**Example (Ollama running on port 11434):**
`http://host.docker.internal:11434/v1`

---

## 🛠️ Tech Stack
- **Backend**: Go (Gin Framework), GORM, SQLite (Persistent container mount)
- **Frontend**: Vanilla HTML/JS/CSS (No build step required), Mobile-Responsive, Glassmorphism UI
- **Deployment**: Docker, Docker Compose

## 📄 License
This project is licensed under the **GNU General Public License v3.0 (GPLv3)**. See the `LICENSE` file for details.
