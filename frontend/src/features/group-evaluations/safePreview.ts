import DOMPurify from 'dompurify'

export const PREVIEW_CSP = "default-src 'none'; script-src 'none'; style-src 'unsafe-inline'; img-src data:; connect-src 'none'; frame-src 'none'; object-src 'none'; form-action 'none'; base-uri 'none'"

const purifier = DOMPurify(window)
purifier.addHook('uponSanitizeAttribute', (_node, attribute) => {
  if (['src', 'poster', 'background'].includes(attribute.attrName) && !/^data:image\/(?:png|gif|jpeg|webp);base64,/i.test(attribute.attrValue)) {
    attribute.keepAttr = false
  }
})

export function buildSafePreview(html: string): string {
  // Remove active navigation as CSP alone does not block a frame navigating itself.
  const content = purifier.sanitize(html, {
    USE_PROFILES: { html: true, svg: true, svgFilters: true },
    FORCE_BODY: true,
    FORBID_TAGS: ['script', 'iframe', 'frame', 'object', 'embed', 'link', 'meta', 'base', 'form', 'foreignObject', 'animate', 'set'],
    FORBID_ATTR: ['href', 'xlink:href', 'action', 'formaction', 'target', 'srcset'],
    ALLOW_DATA_ATTR: false,
  })
  return `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="${PREVIEW_CSP}"><meta name="referrer" content="no-referrer"><style>html,body{margin:0;min-height:100%;background:#fff}body{overflow-wrap:anywhere}svg,img{max-width:100%;height:auto}</style></head><body>${content}</body></html>`
}
