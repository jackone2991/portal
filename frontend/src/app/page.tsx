export default function HomePage() {
  return (
    <main className="mx-auto max-w-3xl px-6 py-24">
      <h1 className="text-4xl font-semibold tracking-tight">Portal</h1>
      <p className="mt-4 text-lg opacity-80">
        Self-hosted media platform — movies, music, stories.
      </p>
      <ul className="mt-10 space-y-2 text-sm opacity-70">
        <li>Frontend: Next.js 15 + Tailwind v4</li>
        <li>Backend: Go (Chi + sqlc + Asynq)</li>
        <li>Storage: MinIO + Cloudflare R2</li>
        <li>Auth: Authentik (OIDC)</li>
      </ul>
    </main>
  );
}
