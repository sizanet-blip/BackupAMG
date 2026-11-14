package secrets

import "errors"

var errNotEncrypted = errors.New("la contraseña guardada no está cifrada. Ejecuta agent set-password en este equipo")
