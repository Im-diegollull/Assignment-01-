// Package web expone los assets estáticos y las plantillas HTML embebidos en
// el binario, de modo que el servidor pueda ejecutarse desde cualquier
// directorio sin depender de rutas relativas en disco.
package web

import "embed"

// Files contiene las plantillas (templates/...) y los assets (static/...).
//
//go:embed templates static
var Files embed.FS
