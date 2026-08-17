<div align="center">
  <h1>NexusAPI Flow 🌊</h1>
  <p><b>Unified LLM API Key Management and Smart Proxy Gateway</b></p>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go">
  <img alt="Gin" src="https://img.shields.io/badge/Gin-Framework-0088cc?style=flat-square">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-Supported-2496ED?style=flat-square&logo=docker">
  
  <p>
    <a href="#english">English</a> | 
    <a href="#中文">中文</a> | 
    <a href="#日本語">日本語</a> | 
    <a href="#русский">Русский</a>
  </p>
</div>

---
> ⚠️ **DISCLAIMER / 免责声明**: The functionality of this project is not fully implemented and is currently intended for testing purposes only. 本项目功能尚未完全实现，目前仅供开发与测试使用。
---

<h2 id="english">🇬🇧 English</h2>

### 📖 Overview
**NexusAPI Flow** is an advanced API Key management platform and smart proxy redistributor designed for Large Language Models (LLMs). It seamlessly unifies multiple upstream LLM providers (OpenAI, Anthropic, Gemini, etc.) behind a single standard `/v1/chat/completions` endpoint. 

Whether you're managing keys for a team, distributing tokens to users, or building a robust failover gateway for production, NexusAPI offers a powerful, elegant, and lightning-fast solution.

### ✨ Core Features
* **🔌 Unified Proxy Gateway**: Transforms various backend models into a standard OpenAI-compatible API format (with full SSE streaming support).
* **🧠 Smart Routing**:
  * **429 Fallback**: Automatically switches to the next available key if the current one hits a rate limit.
  * **Context Scaling**: Automatically upgrades to models with larger context windows if token length is exceeded.
* **👥 Multi-Tenant Architecture**: Complete user management system with Admin and Regular User roles.
* **🔐 Robust API Key Ecosystem**: Global Pool (Admin-managed) & Private Pool (User-managed).
* **📊 Granular Usage Quotas**: Track and limit usage per user by Total, Monthly, Weekly, and rolling 5-Hour windows.
* **🛡️ Security First**: Includes built-in JWT authentication, Bcrypt password hashing, and 2FA (TOTP) support.
* **🌍 True Localization (i18n)**: Out-of-the-box support for 7 languages with a sleek Glassmorphism UI.

### 🚀 Quick Start & Remote Deployment
NexusAPI Flow is designed for effortless remote server deployment via Docker Compose.

**Step-by-Step Installation:**
1. **Clone the repository:**
   ```bash
   git clone https://github.com/ysnsk111/nexus-api.git
   cd nexus-api
   ```
2. **Run the Interactive Setup Script:**
   ```bash
   chmod +x install.sh
   ./install.sh
   ```
3. **Follow the Prompts:** The script will ask for Admin Username, Password, and Panel Port.
4. **Enjoy!** Access your dashboard at: `http://<your-server-ip>:<Port>` (e.g., `http://localhost:8080`).

### 🌐 Advanced Configuration (Nginx Reverse Proxy)
For exposing NexusAPI Flow to the public internet via a sub-path (`/nexus/`):
```nginx
server {
    listen 80;
    server_name api.yourdomain.com;
    location /nexus/ {
        proxy_pass http://127.0.0.1:8080/; 
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

### 🔗 Connecting to Local/Internal APIs (e.g., Ollama)
If routing to an AI model on the host server, use the network bridge hostname:
* **Example:** `http://host.docker.internal:11434/v1`

---

<h2 id="中文">🇨🇳 中文</h2>

### 📖 简介
**NexusAPI Flow** 是一款专为大语言模型（LLM）设计的高级 API 密钥管理平台和智能代理网关。它将多个上游大模型提供商（OpenAI、Anthropic、Gemini等）无缝统一在标准的 `/v1/chat/completions` 接口之下。

### ✨ 核心功能
* **🔌 统一代理网关**：将各类后端模型转换为标准的 OpenAI 兼容格式（支持完整的 SSE 流式输出）。
* **🧠 智能路由**：
  * **429 故障转移**：遇到请求频率限制时自动切换到下一个可用密钥。
  * **上下文自适应**：当 Token 超出限制时，自动切换至具备更大上下文窗口的模型。
* **👥 多租户架构**：具备完整的管理员与普通用户角色管理系统。
* **🔐 强大的 API 密钥生态**：支持全局密钥池（管理员）和私有密钥池（用户自定义）。
* **📊 精细的配额管理**：按总额、月度、每周、以及滚动的 5 小时窗口限制用户配额。
* **🛡️ 安全第一**：内置 JWT 身份验证、Bcrypt 密码哈希以及两步验证（TOTP）。
* **🌍 原生国际化**：面板自带 7 种语言支持，采用现代化的毛玻璃（Glassmorphism）UI 设计。

### 🚀 快速开始与远程部署
通过 Docker Compose，你可以轻松在远程服务器上部署 NexusAPI Flow。

**安装步骤：**
1. **克隆仓库：**
   ```bash
   git clone https://github.com/ysnsk111/nexus-api.git
   cd nexus-api
   ```
2. **运行交互式安装脚本：**
   ```bash
   chmod +x install.sh
   ./install.sh
   ```
3. **根据提示操作：** 脚本会提示你设置管理员账号、密码以及面板映射端口。
4. **大功告成！** 访问 `http://<服务器IP>:<端口>`（例如 `http://localhost:8080`）进入你的专属面板。

### 🌐 Nginx 反向代理配置最佳实践
如果你想在公网通过子路径（如 `/nexus/`）暴露面板，请参考以下配置，并注意务必关闭缓冲以支持 SSE 流式传输：
```nginx
    location /nexus/ {
        proxy_pass http://127.0.0.1:8080/; 
        proxy_set_header Host $host;
        # ...省略其他常规header...
        
        # 必须添加以下配置以支持流式传输
        proxy_set_header Connection '';
        proxy_http_version 1.1;
        chunked_transfer_encoding off;
        proxy_buffering off;
        proxy_cache off;
    }
```

### 🔗 宿主机内网接口打通（如连接 Ollama）
如果上游模型部署在同一台宿主机上（非Docker容器内），请不要使用 `localhost`。请使用 Docker 内置桥接域名：
* **示例：** `http://host.docker.internal:11434/v1`

---

<h2 id="日本語">🇯🇵 日本語</h2>

### 📖 概要
**NexusAPI Flow** は、大規模言語モデル（LLM）のために設計された高度な API キー管理プラットフォームおよびスマートプロキシゲートウェイです。複数のアップストリーム LLM プロバイダーを単一の標準 `/v1/chat/completions` エンドポイントにシームレスに統合します。

### ✨ 主な機能
* **🔌 統合プロキシゲートウェイ**: バックエンドモデルを標準の OpenAI 互換 API 形式に変換（完全な SSE ストリーミング対応）。
* **🧠 スマートルーティング**: レート制限時の自動フォールバック機能やコンテキストサイズ超過時の自動モデルスケールアップ機能。
* **👥 マルチテナントアーキテクチャ**: 管理者および一般ユーザーのための完全な管理システム。
* **📊 詳細な利用枠管理**: ユーザーごとのトークン制限（無期限、月間、週間、5時間ごと）。
* **🛡️ セキュリティ重視**: JWT 認証、Bcrypt パスワードハッシュ、TOTP (二段階認証) を標準搭載。

### 🚀 リモートデプロイとインストール
1. **リポジトリのクローン:**
   ```bash
   git clone https://github.com/ysnsk111/nexus-api.git
   cd nexus-api
   ```
2. **セットアップスクリプトの実行:**
   ```bash
   chmod +x install.sh
   ./install.sh
   ```
3. 管理者情報とポート番号を入力し、デプロイ完了後に指定したポートでアクセスしてください。

*ローカルホスト上の API（Ollamaなど）にアクセスする場合は、`localhost` ではなく `http://host.docker.internal:<ポート>` をベースURLとして使用してください。*

---

<h2 id="русский">🇷🇺 Русский</h2>

### 📖 Обзор
**NexusAPI Flow** — это передовая платформа для управления API-ключами и интеллектуальный прокси-шлюз для больших языковых моделей (LLM). Она объединяет различные LLM (OpenAI, Anthropic, Gemini и др.) в единый стандартный интерфейс `/v1/chat/completions`.

### ✨ Основные функции
* **🔌 Единый прокси-шлюз**: Преобразует запросы к различным моделям в формат, совместимый с OpenAI (с поддержкой SSE стриминга).
* **🧠 Умная маршрутизация**: Автоматическое переключение при ошибках 429 и превышении лимита контекста.
* **👥 Многопользовательская архитектура**: Полное управление администраторами и пользователями.
* **📊 Детальные квоты**: Лимиты по токенам (за все время, месяц, неделю, каждые 5 часов).
* **🛡️ Безопасность**: Встроенная JWT аутентификация, хеширование Bcrypt, поддержка 2FA (TOTP).

### 🚀 Установка и развертывание на сервере
1. **Клонирование репозитория:**
   ```bash
   git clone https://github.com/ysnsk111/nexus-api.git
   cd nexus-api
   ```
2. **Запуск скрипта установки:**
   ```bash
   chmod +x install.sh
   ./install.sh
   ```
3. Введите данные для учетной записи администратора и порт панели.

*Для подключения к API на хост-машине (например, Ollama), используйте адрес `http://host.docker.internal:<PORT>` вместо `localhost`.*
