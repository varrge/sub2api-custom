/** Download a result into a bounded reference file without forwarding account credentials. */
export async function resultImageFile(src: string, maxBytes: number): Promise<File> {
  const url = new URL(src)
  if (!['http:', 'https:'].includes(url.protocol) &&
      !(url.protocol === 'data:' && /^data:image\/(png|jpeg|webp);base64,/i.test(src))) {
    throw new Error('imageGeneration.referenceLoadFailed')
  }
  if (url.protocol === 'data:' && src.length > Math.ceil(maxBytes / 3) * 4 + 64) {
    throw new Error('imageGeneration.referenceTooLarge')
  }
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), 30_000)
  try {
    const response = await fetch(src, { credentials: 'omit', referrerPolicy: 'no-referrer', signal: controller.signal })
    if (!response.ok) throw new Error('imageGeneration.referenceLoadFailed')
    const type = response.headers.get('content-type')?.split(';')[0]?.trim().toLowerCase() || ''
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(type)) throw new Error('imageGeneration.invalidFileType')
    if (Number(response.headers.get('content-length')) > maxBytes) throw new Error('imageGeneration.referenceTooLarge')
    const reader = response.body?.getReader()
    if (!reader) throw new Error('imageGeneration.referenceLoadFailed')
    const chunks: Uint8Array[] = []
    let bytes = 0
    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        bytes += value.byteLength
        if (bytes > maxBytes) throw new Error('imageGeneration.referenceTooLarge')
        chunks.push(value)
      }
    } finally { await reader.cancel(); reader.releaseLock() }
    if (!bytes) throw new Error('imageGeneration.referenceLoadFailed')
    return new File(chunks, `reference-${Date.now()}.${type === 'image/jpeg' ? 'jpg' : type.split('/')[1]}`, { type })
  } finally { clearTimeout(timer) }
}
