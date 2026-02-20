# ingitdb.com

Static promotional website for [inGitDB](https://github.com/ingitdb/ingitdb-cli) - A developer-grade, schema-validated, AI-native database whose storage engine is a Git repository.

## Features

- 🎨 Beautiful, modern design with dark/light theme support
- 📱 Fully responsive layout
- ⚡ Lightweight - no external dependencies for CSS/JS
- 🚀 Optimized for Firebase Hosting
- 📝 Comprehensive documentation
- 🤖 SEO-friendly with robots.txt and proper metadata

## Structure

```
public/
├── index.html          # Landing page
├── docs/
│   └── index.html     # Documentation page
├── css/
│   ├── style.css      # Main styles
│   └── docs.css       # Documentation-specific styles
├── js/
│   ├── theme.js       # Theme toggle functionality
│   └── docs.js        # Documentation navigation
├── favicon.svg        # Site icon
└── robots.txt         # SEO configuration
```

## Local Development

To test the website locally, you can use any static file server:

```bash
# Using Python
cd public
python -m http.server 8000

# Using Node.js
npx http-server public -p 8000

# Using Firebase Hosting emulator
firebase emulators:start --only hosting:ingitdb-com --config ../firebase.json
```

Then open http://localhost:8000 in your browser.

## Deployment

This site is configured for Firebase Hosting. To deploy:

```bash
# Install Firebase CLI if not already installed
npm install -g firebase-tools

# Login to Firebase
firebase login

# Deploy to Firebase Hosting
firebase deploy --only hosting:ingitdb-com --config ../firebase.json
```

## Content

The website content is extracted from the [ingitdb-cli repository](https://github.com/ingitdb/ingitdb-cli):
- Main README for landing page content
- Documentation from the docs/ directory

## License

MIT License - see [LICENSE](LICENSE) file for details.
