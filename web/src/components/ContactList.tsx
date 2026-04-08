import { useStore } from '../store'

interface Contact {
  id: string
  name: string
}

interface Props {
  contacts: Contact[]
  onSelect: (id: string, name: string) => void
  selectedId: string | null
}

export function ContactList({ contacts, onSelect, selectedId }: Props) {
  const presence = useStore((s) => s.presence)

  return (
    <div className="h-full bg-gray-900 border-r border-gray-800 flex flex-col">
      <div className="px-4 py-3 border-b border-gray-800 font-semibold text-white text-sm uppercase tracking-wide">
        Contacts
      </div>
      <div className="flex-1 overflow-y-auto">
        {contacts.map((c) => (
          <button
            key={c.id}
            onClick={() => onSelect(c.id, c.name)}
            className={`w-full flex items-center gap-3 px-4 py-3 hover:bg-gray-800 transition text-left ${selectedId === c.id ? 'bg-gray-800' : ''}`}
          >
            <div className="relative">
              <div className="w-8 h-8 rounded-full bg-indigo-600 flex items-center justify-center text-white text-sm font-bold">
                {c.name[0].toUpperCase()}
              </div>
              <span className={`absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full border-2 border-gray-900 ${presence[c.id] ? 'bg-green-400' : 'bg-gray-500'}`} />
            </div>
            <div>
              <p className="text-sm text-white font-medium">{c.name}</p>
              <p className="text-xs text-gray-400">{presence[c.id] ? 'Online' : 'Offline'}</p>
            </div>
          </button>
        ))}
      </div>
    </div>
  )
}
