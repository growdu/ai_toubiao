// JWT helpers shared across the front-end. `decodeJWT` deliberately does
// not verify the signature — this is a UI convenience to read the
// `tenant_id` claim so we know which tenant the logged-in user belongs
// to. All server-side checks happen in the api-gateway, never trust
// client-claimed JWTs.
export function decodeJWT(token: string): Record<string, unknown> {
  try {
    const payload = token.split('.')[1]
    if (!payload) return {}
    const decoded = atob(payload.replace(/-/g, '+').replace(/_/g, '/'))
    return JSON.parse(decoded)
  } catch {
    return {}
  }
}
