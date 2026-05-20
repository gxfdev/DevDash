import DOMPurify from 'dompurify'

DOMPurify.addHook('uponSanitizeElement', (node, data) => {
  if (data.tagName === 'script') {
    const parent = node.parentNode
    if (parent) {
      parent.removeChild(node)
    }
  }
})

export function sanitizeHtml(dirty: string): string {
  return DOMPurify.sanitize(dirty, {
    ALLOWED_TAGS: ['b', 'i', 'em', 'strong', 'a', 'p', 'br', 'ul', 'ol', 'li', 'code', 'pre'],
    ALLOWED_ATTR: ['href', 'target'],
    ALLOW_DATA_ATTR: false,
  })
}

export function sanitizeText(input: string): string {
  const div = document.createElement('div')
  div.textContent = input
  return div.innerHTML
}
