// PDF.js needs its WASM image decoders (JPEG 2000, JBIG2), colour profiles, CMaps and standard
// fonts at runtime. Without them, scanned books and many other PDFs render as blank white pages.
// Copy them next to the app so the embedded IDIOS server can serve them from /pdfjs/.
import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const source = join(root, 'node_modules', 'pdfjs-dist')
const target = join(root, 'public', 'pdfjs')
rmSync(target, { recursive: true, force: true })
mkdirSync(target, { recursive: true })
for (const name of ['wasm', 'cmaps', 'standard_fonts', 'iccs']) {
  const from = join(source, name)
  if (!existsSync(from)) throw new Error(`pdfjs-dist is missing its "${name}" folder: ${from}`)
  cpSync(from, join(target, name), { recursive: true })
}
console.log('Copied PDF.js runtime assets to public/pdfjs')
