import api from './client'

export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  token: string
  user_id: string
  tenant_id: string
}

export const authApi = {
  login: (data: LoginRequest) => api.post<LoginResponse>('/auth/login', data),
}