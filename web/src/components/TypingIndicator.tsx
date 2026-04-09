export function TypingIndicator({ name }: { name?: string }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 10,
      paddingLeft: 4, marginBottom: 4,
    }}>
      <div style={{
        display: 'inline-flex', alignItems: 'center', gap: 5,
        background: 'rgba(255,255,255,0.055)',
        border: '1px solid rgba(99,102,241,0.14)',
        borderRadius: '18px 18px 18px 4px',
        padding: '10px 14px',
      }}>
        {[0, 150, 300].map((delay) => (
          <span key={delay} style={{
            width: 6, height: 6, borderRadius: '50%',
            background: '#818cf8',
            boxShadow: '0 0 8px rgba(99,102,241,0.8)',
            display: 'inline-block',
            animation: `bounce-dot 1.2s ease-in-out ${delay}ms infinite`,
          }} />
        ))}
      </div>
      {name && (
        <span style={{ fontSize: 11, color: 'rgba(255,255,255,0.3)', letterSpacing: 0.3 }}>
          {name} is typing...
        </span>
      )}
    </div>
  )
}
