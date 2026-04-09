import { useState } from 'react'
import { register, login } from '../api'
import { useStore } from '../store'

export function LoginForm() {
  const setAuth = useStore((s) => s.setAuth)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(action: 'login' | 'register') {
    setLoading(true)
    setError('')
    try {
      const res = action === 'login'
        ? await login(username, password)
        : await register(username, password)
      setAuth(res.token, res.user_id, res.username)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-950">
      <div className="bg-gray-900 rounded-2xl p-8 w-full max-w-sm shadow-xl">
        <h1 className="text-2xl font-bold text-white mb-6 text-center">P2P Chat</h1>
        <div className="space-y-4">
          <input
            className="w-full bg-gray-800 text-white rounded-lg px-4 py-2 outline-none focus:ring-2 focus:ring-indigo-500"
            placeholder="Username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
          <input
            type="password"
            className="w-full bg-gray-800 text-white rounded-lg px-4 py-2 outline-none focus:ring-2 focus:ring-indigo-500"
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSubmit('login')}
          />
          {error && <p className="text-red-400 text-sm">{error}</p>}
          {loading && (
            <p className="text-gray-400 text-xs text-center">Connecting to server, please wait...</p>
          )}
          <div className="flex gap-2">
            <button
              onClick={() => handleSubmit('login')}
              disabled={loading}
              className="flex-1 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white rounded-lg py-2 font-medium transition"
            >
              {loading ? '...' : 'Login'}
            </button>
            <button
              onClick={() => handleSubmit('register')}
              disabled={loading}
              className="flex-1 bg-gray-700 hover:bg-gray-600 disabled:opacity-50 text-white rounded-lg py-2 font-medium transition"
            >
              {loading ? '...' : 'Register'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
