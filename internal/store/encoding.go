package store

import "bytes"

// utf8BOM is the UTF-8 byte order mark.
//
// .kron/config.toml and projects.json are user-editable files. Windows
// editors (and PowerShell 5.1 `Set-Content -Encoding UTF8`) prepend a BOM
// when they save, which makes encoding/json and the minimal TOML parser
// reject the first token. Readers tolerate it; serializers never write it
// back, so the BOM disappears the next time Kron saves the file.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// trimUTF8BOM removes one leading UTF-8 BOM, if present.
func trimUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, utf8BOM)
}
