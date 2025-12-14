# ALEX Web Frontend - Quick Start Guide

Get the ALEX web interface up and running in 5 minutes.

## Prerequisites

- Node.js 20+ and npm
- ALEX backend server (see main README)

## Installation Steps

### 1. Install Dependencies

```bash
cd web
npm install
```

This will install all required packages (~400MB):
- Next.js (App Router)
- React
- TypeScript
- Tailwind CSS
- React Query
- Zustand
- And more...

### 2. Configure Environment

```bash
cp .env.local.example .env.local
```

Edit `.env.local`:

```env
# Point to your ALEX backend server
NEXT_PUBLIC_API_URL=http://localhost:8080
```

**Important**: The backend server must be running and accessible at this URL.

### 3. Start Development Server

```bash
npm run dev
```

You should see:

```
▲ Next.js
- Local:        http://localhost:3000
- Ready in 2.3s
```

### 4. Open in Browser

Navigate to: **http://localhost:3000**

You should see the ALEX home page with:
- Header with logo and navigation
- Task input form
- Empty state message

## Verify Setup

### Test Backend Connection

1. Open browser console (F12)
2. Submit a test task: "List files in current directory"
3. Watch console for SSE connection logs:

```
[SSE] Connected to session: abc123...
```

4. You should see events streaming in real-time

### Check API Connectivity

```bash
# In another terminal
curl http://localhost:8080/health
```

Should return:
```json
{"status":"ok"}
```

## Common Commands

```bash
# Development
npm run dev          # Start dev server (hot reload)

# Production
npm run build        # Build for production
npm start            # Start production server

# Code Quality
npm run lint         # Run ESLint
```

## Troubleshooting

### Port 3000 Already in Use

```bash
# Use different port
PORT=3001 npm run dev
```

### Backend Not Reachable

1. Verify backend is running: `ps aux | grep alex-server`
2. Check backend logs for errors
3. Verify CORS headers on backend
4. Try curl: `curl http://localhost:8080/api/sessions`

### SSE Connection Fails

**Check browser console for errors:**

```javascript
EventSource failed: ERR_CONNECTION_REFUSED
```

**Solutions:**
- Ensure backend `/api/sse` endpoint exists
- Check backend CORS configuration
- Verify session ID is valid

### Module Not Found Errors

```bash
# Clear cache and reinstall
rm -rf node_modules .next
npm install
```

### TypeScript Errors

```bash
# Regenerate types
npm run build
```

## Project Structure Overview

```
web/
├── app/              # Pages (Next.js App Router)
├── components/       # React components
├── hooks/            # Custom hooks
├── lib/              # Core utilities
└── package.json      # Dependencies
```

## Next Steps

1. **Try a Task**: Submit "Create a hello world Python script"
2. **View Sessions**: Click "Sessions" in the header
3. **Explore Events**: Watch real-time tool execution
4. **Read Docs**: Check `README.md` for detailed guide

## Development Tips

### Hot Reload

Changes to files trigger automatic reload:
- Components update instantly
- Pages refresh automatically
- Styles recompile on save

### Browser DevTools

Essential tabs:
- **Console**: SSE connection logs
- **Network**: API calls and SSE stream
- **React DevTools**: Component state

### VS Code Extensions

Recommended:
- ESLint
- Tailwind CSS IntelliSense
- TypeScript Vue Plugin (Volar)

## Production Deployment

### Build

```bash
npm run build
```

Output in `out/` directory.

### Deploy to Vercel

```bash
npm install -g vercel
vercel deploy
```

Set environment variable in Vercel dashboard:
- `NEXT_PUBLIC_API_URL` = your production backend URL

## Useful Links

- **Next.js Docs**: https://nextjs.org/docs
- **React Query Docs**: https://tanstack.com/query
- **Tailwind Docs**: https://tailwindcss.com/docs
- **Zustand Docs**: https://github.com/pmndrs/zustand

## Support

For issues:
1. Check troubleshooting section above
2. Review browser console logs
3. Check backend server logs
4. Open GitHub issue with details

## What's Next?

- ✅ Basic setup complete
- ⬜ Customize theme colors
- ⬜ Add authentication
- ⬜ Implement dark mode
- ⬜ Add tests
- ⬜ Deploy to production

Happy coding with ALEX! 🤖
