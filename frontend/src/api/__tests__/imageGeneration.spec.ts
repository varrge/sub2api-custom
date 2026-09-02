import { describe, expect, it } from 'vitest'
import { isLikelyImageModel, parseGeneratedImages, supportsImageGeneration } from '../imageGeneration'

describe('image generation helpers', () => {
  it('keeps supported image models and excludes video models', () => {
    expect(isLikelyImageModel('gemini-3-pro-image-preview')).toBe(true)
    expect(isLikelyImageModel('gpt-image-2')).toBe(true)
    expect(isLikelyImageModel('grok-imagine-video-1.5')).toBe(false)
    expect(isLikelyImageModel('imagen-3')).toBe(false)
    expect(supportsImageGeneration('anthropic')).toBe(false)
  })

  it('normalizes OpenAI and Gemini image responses', () => {
    expect(parseGeneratedImages({ data: [{ b64_json: 'QUJD', revised_prompt: 'cat' }] })).toEqual([
      { src: 'data:image/png;base64,QUJD', revisedPrompt: 'cat' },
    ])
    expect(parseGeneratedImages({
      candidates: [{ content: { parts: [{ text: 'done' }, { inlineData: { mimeType: 'image/webp', data: 'REVG' } }] } }],
    })).toEqual([{ src: 'data:image/webp;base64,REVG', revisedPrompt: 'done' }])
  })
})
