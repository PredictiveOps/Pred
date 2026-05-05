# Agent Guidelines

## UI Conventions

These conventions apply to all UI work in the `web-frontend/` directory:

- Use Next.js file-based routing (`app/` directory) for all views — do not use `useState` to swap views. Each view should be its own page/route.
- All clickable elements must be `<button>` elements with `type="button"`. Never use `<div>`, `<span>`, or other non-interactive elements as click targets.
- Follow the existing dark theme — `bg-slate-800`/`bg-slate-900` cards, `text-slate-400` for secondary text, `text-white` for primary
- Status colors: green = normal, yellow/orange = warning, red = critical, blue = info/selected

## Formatting & Linting

Node.js based services (web-frontend) uses Biome instead of ESLint or Prettier.

Go based services (event-processing-service, notifications-service) use Go standard formatting.
