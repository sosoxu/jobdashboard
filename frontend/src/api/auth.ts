import http from './http'
import type { AuthResult, AuthUser } from './types'

export function register(username: string, password: string): Promise<AuthResult> {
  return http.post('/auth/register', { username, password })
}

export function login(username: string, password: string): Promise<AuthResult> {
  return http.post('/auth/login', { username, password })
}

export function getMe(): Promise<AuthUser> {
  return http.get('/auth/me')
}
