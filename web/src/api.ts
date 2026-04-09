const BASE = import.meta.env.VITE_API_URL ?? ''

export interface AuthResponse {
  token: string
  user_id: string
  username: string
}

async function fetchWithRetry(url: string, options: RequestInit, retries = 3): Promise<Response> {
  for (let i = 0; i < retries; i++) {
    try {
      const res = await fetch(url, { ...options, signal: AbortSignal.timeout(15000) })
      return res
    } catch {
      if (i === retries - 1) throw new Error('Server is waking up, please try again in a moment')
      await new Promise((r) => setTimeout(r, 2000))
    }
  }
  throw new Error('Server unreachable')
}

export async function register(username: string, password: string): Promise<AuthResponse> {
  const res = await fetchWithRetry(`${BASE}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) {
    const err = await res.json()
    throw new Error(err.error ?? 'Registration failed')
  }
  return res.json()
}

export async function login(username: string, password: string): Promise<AuthResponse> {
  const res = await fetchWithRetry(`${BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) {
    const err = await res.json()
    throw new Error(err.error ?? 'Login failed')
  }
  return res.json()
}

export interface HistoryMessage {
  id: string
  from: string
  from_id: string
  body: string
  status: string
  timestamp: string
}

export async function fetchHistory(partnerId: string, token: string): Promise<HistoryMessage[]> {
  const res = await fetch(`${BASE}/messages?partner_id=${partnerId}`, {
    headers: { Authorization: `Bearer ${token}` },
    signal: AbortSignal.timeout(10000),
  })
  if (!res.ok) return []
  return res.json()
}
