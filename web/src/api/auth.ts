import api from './client'
import { decodeJWT } from '../lib/jwt'

export interface LoginRequest {
  tenant_slug: string
  email: string
  password: string
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  token_type: string
  user: {
    id: string
    email: string
    role: string
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
