import { useState } from 'react'
import { register, login } from '../api'
import { useStore } from '../store'
import { Zap, User, Lock, LogIn, UserPlus } from 'lucide-react'

export function LoginForm() {
  const setAuth = useStore((s) => s.setAuth)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [mode, setMode] = useState<'login' | 'register'>('login')

  async function handleSubmit(e?: React.FormEvent) {
    e?.preventDefault()
    if (!username.trim() || !password.trim()) {
      setError('Username and password are required')
      return
    }
    setLoading(true)
    setError('')
    try {
      const res = mode === 'login'
        ? await login(username, password)
        : await register(username, password)
      setAuth(res.token, res.user_id, res.username)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center relative overflow-hidden"
      style={{ background: 'linear-gradient(135deg, #0d0d1a 0%, #0a0a14 50%, #0d0d1f 100%)' }}>

      {/* Background glow orbs */}
      <div style={{
        position: 'absolute', width: 400, height: 400, borderRadius: '50%',
        background: 'radial-gradient(circle, rgba(99,102,241,0.12) 0%, transparent 70%)',
        top: '10%', left: '20%', filter: 'blur(40px)', pointerEvents: 'none',
      }} />
      <div style={{
        position: 'absolute', width: 300, height: 300, borderRadius: '50%',
        background: 'radial-gradient(circle, rgba(139,92,246,0.1) 0%, transparent 70%)',
        bottom: '15%', right: '15%', filter: 'blur(40px)', pointerEvents: 'none',
      }} />

      {/* Grid pattern overlay */}
      <div style={{
        position: 'absolute', inset: 0, pointerEvents: 'none',
        backgroundImage: 'linear-gradient(rgba(99,102,241,0.03) 1px, transparent 1px), linear-gradient(90deg, rgba(99,102,241,0.03) 1px, transparent 1px)',
        backgroundSize: '40px 40px',
      }} />

      <div className="animate-slide-up relative w-full max-w-sm mx-4">
        {/* Logo */}
        <div className="flex flex-col items-center mb-8">
          <div style={{
            width: 56, height: 56, borderRadius: 16,
            background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
            boxShadow: '0 0 30px rgba(99,102,241,0.5), 0 0 60px rgba(99,102,241,0.2)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            marginBottom: 16,
          }}>
            <Zap size={28} color="white" fill="white" />
          </div>
          <h1 style={{
            fontFamily: "'Orbitron', monospace", fontSize: 24, fontWeight: 900,
            letterSpacing: 4, color: '#fff',
            textShadow: '0 0 20px rgba(99,102,241,0.8), 0 0 40px rgba(99,102,241,0.4)',
          }}>P2P CHAT</h1>
          <p style={{ color: 'rgba(255,255,255,0.3)', fontSize: 11, letterSpacing: 2, marginTop: 4 }}>
            SECURE · REAL-TIME · DECENTRALIZED
          </p>
        </div>

        {/* Card */}
        <div style={{
          background: 'rgba(255,255,255,0.03)',
          border: '1px solid rgba(99,102,241,0.2)',
          borderRadius: 20,
          padding: '32px 28px',
          backdropFilter: 'blur(20px)',
          boxShadow: '0 0 40px rgba(99,102,241,0.08), inset 0 1px 0 rgba(255,255,255,0.05)',
        }}>

          {/* Tab switcher */}
          <div style={{
            display: 'flex', background: 'rgba(0,0,0,0.3)', borderRadius: 10,
            padding: 3, marginBottom: 24,
          }}>
            {(['login', 'register'] as const).map((m) => (
              <button key={m} onClick={() => { setMode(m); setError('') }}
                style={{
                  flex: 1, padding: '8px 0', borderRadius: 8, fontSize: 11,
                  fontFamily: "'JetBrains Mono', monospace", fontWeight: 600,
                  letterSpacing: 1, cursor: 'pointer', border: 'none',
                  transition: 'all 0.2s ease',
                  background: mode === m ? 'linear-gradient(135deg, #6366f1, #8b5cf6)' : 'transparent',
                  color: mode === m ? 'white' : 'rgba(255,255,255,0.4)',
                  boxShadow: mode === m ? '0 0 15px rgba(99,102,241,0.4)' : 'none',
                  textTransform: 'uppercase',
                }}>
                {m}
              </button>
            ))}
          </div>

          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            {/* Username */}
            <div style={{ position: 'relative' }}>
              <div style={{
                position: 'absolute', left: 14, top: '50%', transform: 'translateY(-50%)',
                color: 'rgba(99,102,241,0.7)', pointerEvents: 'none',
              }}>
                <User size={15} />
              </div>
              <input
                type="text"
                placeholder="Username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
                style={{
                  width: '100%', padding: '12px 14px 12px 40px',
                  background: 'rgba(0,0,0,0.3)',
                  border: '1px solid rgba(99,102,241,0.2)',
                  borderRadius: 10, color: '#ededef', fontSize: 13,
                  fontFamily: "'JetBrains Mono', monospace",
                  outline: 'none', transition: 'border-color 0.2s, box-shadow 0.2s',
                }}
                onFocus={(e) => {
                  e.target.style.borderColor = 'rgba(99,102,241,0.6)'
                  e.target.style.boxShadow = '0 0 0 3px rgba(99,102,241,0.1), 0 0 15px rgba(99,102,241,0.15)'
                }}
                onBlur={(e) => {
                  e.target.style.borderColor = 'rgba(99,102,241,0.2)'
                  e.target.style.boxShadow = 'none'
                }}
              />
            </div>

            {/* Password */}
            <div style={{ position: 'relative' }}>
              <div style={{
                position: 'absolute', left: 14, top: '50%', transform: 'translateY(-50%)',
                color: 'rgba(99,102,241,0.7)', pointerEvents: 'none',
              }}>
                <Lock size={15} />
              </div>
              <input
                type="password"
                placeholder="Password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                style={{
                  width: '100%', padding: '12px 14px 12px 40px',
                  background: 'rgba(0,0,0,0.3)',
                  border: '1px solid rgba(99,102,241,0.2)',
                  borderRadius: 10, color: '#ededef', fontSize: 13,
                  fontFamily: "'JetBrains Mono', monospace",
                  outline: 'none', transition: 'border-color 0.2s, box-shadow 0.2s',
                }}
                onFocus={(e) => {
                  e.target.style.borderColor = 'rgba(99,102,241,0.6)'
                  e.target.style.boxShadow = '0 0 0 3px rgba(99,102,241,0.1), 0 0 15px rgba(99,102,241,0.15)'
                }}
                onBlur={(e) => {
                  e.target.style.borderColor = 'rgba(99,102,241,0.2)'
                  e.target.style.boxShadow = 'none'
                }}
              />
            </div>

            {/* Error */}
            {error && (
              <div style={{
                background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)',
                borderRadius: 8, padding: '8px 12px', fontSize: 11,
                color: '#f87171', letterSpacing: 0.5,
              }}>
                {error}
              </div>
            )}

            {/* Loading hint */}
            {loading && (
              <p style={{ fontSize: 10, color: 'rgba(255,255,255,0.3)', textAlign: 'center', letterSpacing: 1 }}>
                CONNECTING TO SERVER...
              </p>
            )}

            {/* Submit */}
            <button
              type="submit"
              disabled={loading}
              style={{
                display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                padding: '13px 0', borderRadius: 10, border: 'none', cursor: loading ? 'not-allowed' : 'pointer',
                background: loading ? 'rgba(99,102,241,0.3)' : 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                color: 'white', fontSize: 12, fontWeight: 700,
                fontFamily: "'JetBrains Mono', monospace", letterSpacing: 2,
                textTransform: 'uppercase',
                boxShadow: loading ? 'none' : '0 0 20px rgba(99,102,241,0.4), 0 0 40px rgba(99,102,241,0.15)',
                transition: 'all 0.2s ease', marginTop: 4,
              }}
            >
              {loading ? (
                <span style={{ opacity: 0.7 }}>...</span>
              ) : mode === 'login' ? (
                <><LogIn size={14} /> LOGIN</>
              ) : (
                <><UserPlus size={14} /> REGISTER</>
              )}
            </button>
          </form>
        </div>

        <p style={{ textAlign: 'center', marginTop: 20, fontSize: 10, color: 'rgba(255,255,255,0.2)', letterSpacing: 1 }}>
          END-TO-END ENCRYPTED · ZERO KNOWLEDGE
        </p>
      </div>
    </div>
  )
}
