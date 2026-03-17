# CSCAN

**Enterprise Distributed Network Asset Scanning Platform** | Go-Zero + Vue3

[中文](README.md) | [English](README_EN.md)

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat&logo=vue.js)](https://vuejs.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Version](https://img.shields.io/badge/Version-2.7-green)](VERSION)
[![Demo](https://img.shields.io/badge/Demo-Live-orange)](http://cscan.txf7.cn)

## Features

| Module | Function | Tools |
|--------|----------|-------|
| Asset Discovery | Port Scanning, Service Detection | Naabu / Masscan / Nmap |
| Subdomain Enum | Passive Enum + Dictionary Brute | Subfinder + KSubdomain |
| Fingerprinting | Web Fingerprint, 3W+ Rules | Httpx + Wappalyzer + Custom Engine |
| URL Discovery | Path Crawling | Urlfinder |
| Vuln Detection | POC Scanning, Custom POC | Nuclei SDK |
| Web Screenshot | Page Snapshot | Chromedp / HTTPX |
| Online Data Source | API Aggregation Search | FOFA / Hunter / Quake |

**Platform Capabilities**: Distributed Architecture · Multi-Workspace · Report Export · Audit Log

## Quick Start

```bash
git clone https://github.com/tangxiaofeng7/cscan.git
cd cscan

# Linux/macOS
chmod +x cscan.sh && ./cscan.sh

# Windows
.\cscan.bat
```

Access `https://ip:3443`, default account `admin / 123456`

> ⚠️ Worker nodes must be deployed before executing scans

## Project Structure

```
cscan/
├── api/          # HTTP API Service
├── rpc/          # RPC Internal Communication
├── worker/       # Scan Nodes
├── scanner/      # Scan Engine
├── scheduler/    # Task Scheduler
├── model/        # Data Models
├── pkg/          # Common Utilities
├── onlineapi/    # FOFA/Hunter/Quake Integration
├── poc/          # POC Templates
├── web/          # Vue3 Frontend
└── docker/       # Docker Configuration
```

## Local Development

```bash
# 1. Start dependencies
docker-compose -f docker-compose.dev.yaml up -d

# 2. Start services
go run rpc/task/task.go -f rpc/task/etc/task.yaml
go run api/cscan.go -f api/etc/cscan.yaml

# 3. Start frontend
cd web ; npm install ; npm run dev

# 4. Start Worker
go run cmd/worker/main.go -k <install_key> -s http://localhost:8888
```

## Worker Deployment

```bash
# Linux
./cscan-worker -k <install_key> -s http://<api_host>:8888

# Windows
cscan-worker.exe -k <install_key> -s http://<api_host>:8888
```

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.25 + Go-Zero |
| Frontend | Vue 3.4 + Element Plus + Vite + Sass |
| Storage | MongoDB 6 + Redis 7 |
| Scanning | Naabu / Masscan / Nmap / Subfinder / Httpx / Nuclei |

## License

MIT
