interface Props {
  status: string
}

const styles: Record<string, { bg: string; dot: string }> = {
  active: { bg: 'bg-green-100 text-green-800', dot: 'bg-green-500' },
  disabled: { bg: 'bg-gray-100 text-gray-600', dot: 'bg-gray-400' },
  error: { bg: 'bg-red-100 text-red-800', dot: 'bg-red-500' },
}

export default function PluginStatusBadge({ status }: Props) {
  const s = styles[status] ?? styles.disabled
  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${s.bg}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${s.dot}`} />
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  )
}
