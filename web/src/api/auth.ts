import api from './client'

export interface LoginRequest {
  tenant_slug: string
  email: string
  password: string
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
  user: {
    id: string
    email: string
    role: string
  }
}

// decodeJWT extracts the payload from a JWT without verifying the signature.
// Used to get tenant_id from the access token (which is set by the backend
// but not included in the login response body).
function decodeJWT(token: string): Record<string, unknown> {
  try {
    const payload = token.split('.')[1]
    const decoded = atob(payload.replace(/-/g, '+').replace(/_/g, '/'))
    return JSON.parse(decoded)
  } catch {
    return {}
  }
}

export const authApi = {
  login: (data: LoginRequest) => api.post<LoginResponse>('/auth/login', data),

  // extractAuthInfo pulls token, user_id, and tenant_id from the login
  // response. tenant_id is decoded from the JWT claims because the
  // backend doesn't include it as a top-level field.
  extractAuthInfo: (res: { data: LoginResponse }) => {
    const token = res.data.access_token
    const claims = decodeJWT(token)
    return {
      token,
      userId: res.data.user.id,
      tenantId: (claims.tenant_id as string) || '',
    }
  },
}
