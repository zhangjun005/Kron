'use strict'

// A tiny stdio JSON-RPC server used only to exercise vscode-languageclient startup.
let input = Buffer.alloc(0)
let initialized = false

process.stdin.on('data', (chunk) => {
  input = Buffer.concat([input, chunk])
  while (true) {
    const boundary = input.indexOf('\r\n\r\n')
    if (boundary < 0) return
    const header = input.subarray(0, boundary).toString('ascii')
    const lengthLine = header.split('\r\n').find((line) => line.toLowerCase().startsWith('content-length:'))
    const length = Number(lengthLine?.split(':')[1]?.trim())
    if (!Number.isSafeInteger(length) || length < 0) process.exit(1)
    const bodyStart = boundary + 4
    if (input.length < bodyStart + length) return
    const message = JSON.parse(input.subarray(bodyStart, bodyStart + length).toString('utf8'))
    input = input.subarray(bodyStart + length)
    if (message.method === 'initialize') {
      initialized = true
      send({ jsonrpc: '2.0', id: message.id, result: { capabilities: { textDocumentSync: 1 }, serverInfo: { name: 'kron-demo', version: '0.0.1' } } })
    } else if (message.method === 'initialized') {
      if (!initialized) process.exit(1)
    } else if (message.method === 'shutdown') {
      send({ jsonrpc: '2.0', id: message.id, result: null })
    } else if (message.method === 'exit') {
      process.exit(initialized ? 0 : 1)
    } else if (message.id !== undefined) {
      send({ jsonrpc: '2.0', id: message.id, error: { code: -32601, message: 'Method not found' } })
    }
  }
})

function send(message) {
  const body = Buffer.from(JSON.stringify(message), 'utf8')
  process.stdout.write(`Content-Length: ${body.length}\r\n\r\n`)
  process.stdout.write(body)
}
