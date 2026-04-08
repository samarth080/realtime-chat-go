import { useStore } from './store'
import { LoginPage } from './pages/LoginPage'
import { ChatPage } from './pages/ChatPage'

export default function App() {
  const token = useStore((s) => s.token)
  return token ? <ChatPage /> : <LoginPage />
}
