# dialtone.earth

Three.js Earth visualization and landing page for the Dialtone project.

**Live**: [https://dialtone.earth](https://dialtone.earth)

## Development

```bash
# Install dependencies
npm install

# Start dev server
npm run dev

# Build for production
npm run build
```

## Features

- **Earth Globe**: Interactive 3D Earth with atmosphere, clouds, and hex grid overlay
- **Neural Network**: Animated neural network topology visualization
- **Build Curriculum**: Training curriculum visualization
- **Video Section**: Background video showcase

## Tech Stack

- [Vite](https://vitejs.dev/) - Build tool
- [Three.js](https://threejs.org/) - 3D graphics
- [TypeScript](https://www.typescriptlang.org/) - Type safety
- [Vercel](https://vercel.com/) - Hosting

## Pages

- `/` - Home (Earth visualization)
- `/about` - About page
- `/docs` - Documentation

## Deployment

Deployed via the parent plugin. From repo root:

```bash
./dialtone.sh www publish
```

Or manually:

```bash
VERCEL_PROJECT_ID=prj_vynjSZFIhD8TlR8oOyuXTKjFUQxM \
VERCEL_ORG_ID=team_4tzswM6M6PoDxaszH2ZHs5J7 \
vercel deploy --prod
```
