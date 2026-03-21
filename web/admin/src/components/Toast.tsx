import { useCallback, useEffect, useRef, useState } from 'react'

interface ToastMessage {
  id: number
  text: string
  type: 'success' | 'error'
}

type ToastFn = (text: string, type: 'success' | 'error') => void

const subscribers = new Set<ToastFn>()

export function toast(text: string, type: 'success' | 'error' = 'success') {
  subscribers.forEach((fn) => fn(text, type))
}

export default function ToastContainer() {
  const [toasts, setToasts] = useState<ToastMessage[]>([])
  const idRef = useRef(0)

  const add: ToastFn = useCallback((text, type) => {
    const id = ++idRef.current
    setToasts((prev) => [...prev, { id, text, type }])
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 4000)
  }, [])

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  useEffect(() => {
    subscribers.add(add)
    return () => { subscribers.delete(add) }
  }, [add])

  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-sm">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`flex items-start gap-2 px-4 py-3 rounded-lg shadow-lg text-sm text-white animate-fade-in ${
            t.type === 'error' ? 'bg-red-600' : 'bg-green-600'
          }`}
        >
          <span className="flex-1">{t.text}</span>
          <button
            onClick={() => dismiss(t.id)}
            className="text-white/70 hover:text-white shrink-0 leading-none"
            aria-label="Dismiss"
          >
            &times;
          </button>
        </div>
      ))}
    </div>
  )
}
