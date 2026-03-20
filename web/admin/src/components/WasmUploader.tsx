import { useState, useCallback, type DragEvent } from 'react'
import { toast } from './Toast'

interface Props {
  onFile: (file: File) => void
  loading?: boolean
  /** Optional accept hint, defaults to .wasm */
  accept?: string
}

export default function WasmUploader({ onFile, loading, accept = '.wasm' }: Props) {
  const [dragOver, setDragOver] = useState(false)

  const validateAndSubmit = useCallback(
    (file: File | undefined) => {
      if (!file) return
      if (!file.name.endsWith('.wasm')) {
        toast('Only .wasm files are supported', 'error')
        return
      }
      onFile(file)
    },
    [onFile],
  )

  const handleDrop = useCallback(
    (e: DragEvent) => {
      e.preventDefault()
      setDragOver(false)
      validateAndSubmit(e.dataTransfer.files[0])
    },
    [validateAndSubmit],
  )

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    validateAndSubmit(e.target.files?.[0])
    // Reset so same file can be re-selected
    e.target.value = ''
  }

  return (
    <div
      onDragOver={(e) => {
        e.preventDefault()
        if (!loading) setDragOver(true)
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={handleDrop}
      className={`border-2 border-dashed rounded-xl p-8 sm:p-12 text-center transition-colors ${
        dragOver ? 'border-blue-400 bg-blue-50' : 'border-gray-300 bg-white'
      } ${loading ? 'opacity-50 pointer-events-none' : ''}`}
    >
      <div className="text-gray-400 mb-3">
        <svg className="mx-auto h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 16V4m0 0l-4 4m4-4l4 4M4 20h16" />
        </svg>
      </div>
      <p className="text-gray-500 mb-4 text-sm sm:text-base">
        {loading ? 'Uploading...' : 'Drag & drop a .wasm file here, or click to browse'}
      </p>
      <label className="inline-block px-4 py-2 bg-blue-600 text-white rounded-lg cursor-pointer hover:bg-blue-700 text-sm transition-colors">
        Browse
        <input type="file" accept={accept} onChange={handleChange} className="hidden" />
      </label>
    </div>
  )
}
