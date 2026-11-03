# Inquiry Explorer

Copyright (c) 2026 LingXuan-Li

[![Go](https://img.shields.io/badge/Go-1.27.1-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-6C5CE7?style=flat-square)](#)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](https://github.com/LingXuan-Li/InquiryExplorer/blob/main/LICENSE)

![Status](https://img.shields.io/badge/Status-In%20Development-orange?style=flat-square)
[![Formatting and Testing](https://img.shields.io/github/actions/workflow/status/LingXuan-Li/InquiryExplorer/main.yml?branch=main&style=flat-square&logo=github&logoColor=white&label=Formatting%20and%20Testing)](https://github.com/LingXuan-Li/InquiryExplorer/actions/workflows/main.yml)
[![Last Commit](https://img.shields.io/github/last-commit/LingXuan-Li/InquiryExplorer?style=flat-square)](https://github.com/LingXuan-Li/InquiryExplorer/commits/main)

Inquiry Explorer is a desktop application built around a **non-destructive file organization system based on tags and virtual folders**. It is intended to help people organize and explore files without changing the underlying folder structure. Tags and virtual folders provide an organizational layer while the original files stay where they are.

> The tag and virtual-folder workflow describes the project's direction. The current application code includes a desktop shell and a system storage monitor; a complete file-tagging workflow is not yet implemented.

## Current Features

- Cross-platform desktop application built with Go and [Glaze](https://github.com/crgimenes/glaze).
- Storage monitor showing total, used, and free space, usage by mount point, and filesystem details.
- Web-based interface with a sidebar and dynamically loaded pages.
- Setup templates are downloaded from the project's GitHub repository and updated by commit version.

## The Organization Model

Traditional file organization depends on moving files into folders. Inquiry Explorer is designed around a different model:

- **Tags** describe what a file is or how it is used.
- **Virtual folders** collect files using organizational views without requiring a physical move.
- **Non-destructive organization** keeps the original files and directory structure intact while making files easier to find and group.

## Requirements

- Go 1.27.1 or later
- A supported desktop environment for Glaze
- Internet access on first launch and when downloading updated setup templates

## Run from Source

```bash
git clone https://github.com/LingXuan-Li/InquiryExplorer.git
cd InquiryExplorer
go run .
```

The application creates its configuration and data directories under the operating system's user configuration directory. On launch, it checks GitHub for setup template updates.

## Build

```bash
go build -o inquiry-explorer .
```

On Windows, use an `.exe` output name:

```powershell
go build -o inquiry-explorer.exe .
```

## Project Structure

```text
.
├── file/                     # Configuration paths and setup-template downloads
├── function/                 # Go methods exposed to the desktop UI
├── scan/                     # Storage information collection
├── setup/
│   └── templates/            # Application shell and pages
├── main.go                   # Desktop application entry point
└── go.mod
```

## Contact

- **GitHub**: [![GitHub](https://img.shields.io/badge/GitHub-181717?style=flat-square&logo=github&logoColor=white)](https://github.com/LingXuan-Li)
- **Twitter/X**: [![Twitter/X](https://img.shields.io/badge/Twitter/X-1DA1F2?style=flat-square&logo=x&logoColor=white)](https://x.com/RaccoonDog_329)

Please submit bug reports via [GitHub Issues](https://github.com/LingXuan-Li/InquiryExplorer/issues) and feel free to reach out with any questions on [Twitter/X](https://x.com/RaccoonDog_329).
