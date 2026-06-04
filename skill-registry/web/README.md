# Skill Registry Web UI

Modern React web interface for browsing and managing skills in the Skill Registry.

## Features

- 🔍 Search and filter skills
- 📦 Browse skill catalog
- 📊 View skill details and versions
- ⬇️ Download skill packages
- 📈 Registry statistics

## Development

```bash
# Install dependencies
npm install

# Start development server (with API proxy)
npm run dev

# Build for production
npm run build
```

The dev server runs on `http://localhost:3000` and proxies API requests to `http://localhost:8080`.

## Production Deployment

1. Build the static files:
   ```bash
   npm run build
   ```

2. The output will be in `dist/` directory

3. Serve the static files from your Go server (see main README)

## Tech Stack

- React 18
- Vite
- React Router
- Fetch API for HTTP requests
