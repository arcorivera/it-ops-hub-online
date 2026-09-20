// Resolves the backend API's base URL.
//
// If NEXT_PUBLIC_API_BASE is set at build time, that always wins (useful for
// local development against a specific port). Otherwise, at runtime in the
// browser, this derives the API host from whatever host the page was loaded
// from — so opening the app via http://localhost:3000 talks to
// http://localhost:8080, and opening it via http://192.168.1.50:3000 (e.g.
// from a phone on the same network) talks to http://192.168.1.50:8080,
// with no rebuild and no per-machine config needed.
export function getApiBase(): string {
  if (process.env.NEXT_PUBLIC_API_BASE) {
    return process.env.NEXT_PUBLIC_API_BASE;
  }
  if (typeof window !== "undefined") {
    return `${window.location.protocol}//${window.location.hostname}:8080`;
  }
  return "http://localhost:8080";
}
