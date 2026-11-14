package config

import "errors"

// errAlreadyConfigured impide que el asistente borre una configuración que ya funciona.
var errAlreadyConfigured = errors.New("ya hay una configuración")
