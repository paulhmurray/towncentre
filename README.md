# TownCentre

TownCentre is an eCommerce application built with GoLang, HTMX, TailwindCSS, and MariaDB. It powers a modern, lightweight online store with a focus on merchant usability and fast performance. Deployed on a Digital Ocean VPS, it leverages Nginx for reverse proxying, systemd for service management, and an automated GitHub polling system for continuous deployment.

### Live at: towncentre.au

## Features

    Backend: GoLang for a robust and efficient server.
    Frontend: HTMX for dynamic interactivity and TailwindCSS for responsive styling.
    Database: MariaDB for reliable data storage.
    Deployment: Automated updates via cron job polling GitHub every minute.
    Email: Forwarding for support@towncentre.au to a personal inbox via Forward Email.
    Security: SSL via Certbot with Nginx.

## Project Structure

text
towncentre/
├── cmd/ # Entry points
│ └── web/ # Main web application
│ └── main.go # Application entry
├── internal/ # Private application code
│ ├── handlers/ # HTTP request handlers
│ └── models/ # Data models
├── ui/ # Frontend assets
│ ├── assets/ # Static assets (e.g., images)
│ ├── html/ # HTMX templates
│ │ ├── pages/ # Full page templates
│ │ └── partials/ # Reusable snippets
│ └── static/ # Compiled CSS and JS
│ ├── css/ # TailwindCSS input/output
│ └── js/ # Client-side scripts
├── towncentre # Compiled binary (ignored in git)
└── README.md # This file

## Prerequisites

    Go: 1.21+ (for building the backend)
    Node.js: 18+ (for TailwindCSS)
    npm: 9+ (for managing frontend dependencies)
    MariaDB: 10.5+ (for the database)
    Digital Ocean VPS: Ubuntu 22.04+ (or similar Linux distro)
    Nginx: 1.18+ (for reverse proxy)
    Git: For version control

## Local Development

    Clone the Repo:
    bash

git clone https://github.com/paulhmurray/towncentre.git
cd towncentre
Install Go Dependencies:
bash
go mod tidy
Install Frontend Dependencies:
bash
npm install
Build TailwindCSS:
bash
npm run dev # Watch mode for development

# OR

npm run build # Minified production build
Run the App:
bash

    go run ./cmd/web
        Access at http://localhost:4000.
    Database Setup:
        Configure MariaDB with your schema (TBD—add your migration instructions here).

## Deployment

The app is deployed on a Digital Ocean VPS with the following setup:
Server Setup

    Clone the Repo:
    bash

mkdir -p /home/towncentre
git clone https://github.com/paulhmurray/towncentre.git /home/towncentre/towncentre
cd /home/towncentre/towncentre
Build the Binary:
bash
go mod tidy
go build -o towncentre ./cmd/web
Install Nginx:
bash
sudo apt install nginx -y

    Configure /etc/nginx/sites-available/towncentre (see repo for config).

Set Up Systemd Service:

    Create /etc/systemd/system/towncentre.service (see repo for contents).
    Enable and start:
    bash

    sudo systemctl daemon-reload
    sudo systemctl enable towncentre
    sudo systemctl start towncentre

SSL with Certbot:
bash
sudo apt install certbot python3-certbot-nginx -y
sudo certbot --nginx -d towncentre.au -d www.towncentre.au

### Permissions:

bash

    sudo chown -R towncentre:www-data /home/towncentre/towncentre/ui/static
    sudo chmod 755 /home/towncentre /home/towncentre/towncentre /home/towncentre/towncentre/ui /home/towncentre/towncentre/ui/static
    sudo chmod 644 /home/towncentre/towncentre/ui/static/css/*

### Continuous Deployment

    A cron job polls GitHub every minute and rebuilds on changes:
        Script: /usr/local/bin/poll_and_rebuild.sh (see repo for contents).
        Cron: * * * * * /usr/local/bin/poll_and_rebuild.sh (runs as towncentre user).
        Requires sudo privileges for systemctl (configured via /etc/sudoers.d/towncentre).

### Email Forwarding

    Provider: Forward Email (free tier).
    Alias: support@towncentre.au → personal Gmail.
    DNS (Digital Ocean):
        MX: 10 mx1.forwardemail.net, 10 mx2.forwardemail.net
        TXT: @ "forward-email=support:yourname@gmail.com"
        TXT: @ "v=spf1 a mx include:spf.forwardemail.net ~all"

## Contributing

    Fork the repo, make changes, and submit a PR.
    Issues? Open a ticket at github.com/paulhmurray/towncentre/issues.

## Contact

    Reach out at support@towncentre.au.

## License

ISC © Paul Murray
