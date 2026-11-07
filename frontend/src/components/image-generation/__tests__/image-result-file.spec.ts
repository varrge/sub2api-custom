import { afterEach, describe, expect, it, vi } from 'vitest'
import { resultImageFile } from '../image-result-file'

afterEach(() => vi.unstubAllGlobals())

describe('generated image references', () => {
  it('reads a supported image without forwarding credentials or the referrer', async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(new Uint8Array([1, 2, 3]), { headers: { 'content-type': 'image/webp' } }))
    vi.stubGlobal('fetch', fetcher)
    const file = await resultImageFile('https://media.example/result.webp', 100)
    expect(file.type).toBe('image/webp')
    expect(file.size).toBe(3)
    expect(fetcher).toHaveBeenCalledWith('https://media.example/result.webp', expect.objectContaining({ credentials: 'omit', referrerPolicy: 'no-referrer' }))
  })
  it('rejects unexpected protocols and oversized data before a download', async () => {
    const fetcher = vi.fn()
    vi.stubGlobal('fetch', fetcher)
    await expect(resultImageFile('javascript:alert(1)', 10)).rejects.toThrow('referenceLoadFailed')
    await expect(resultImageFile('data:image/svg+xml;base64,AAAA', 10)).rejects.toThrow('referenceLoadFailed')
    await expect(resultImageFile('data:image/png;base64,' + 'A'.repeat(200), 10)).rejects.toThrow('referenceTooLarge')
    expect(fetcher).not.toHaveBeenCalled()
  })
  it('bounds streaming downloads even when the size header is missing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(new Uint8Array(12), { headers: { 'content-type': 'image/png' } })))
    await expect(resultImageFile('https://media.example/image.png', 10)).rejects.toThrow('referenceTooLarge')
  })
  it('rejects HTML error bodies served with a successful status', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('<html>expired</html>', { headers: { 'content-type': 'text/html' } })))
    await expect(resultImageFile('https://media.example/image.png', 100)).rejects.toThrow('invalidFileType')
  })
})
