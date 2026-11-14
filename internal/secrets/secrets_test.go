package secrets

import (
	"strings"
	"testing"
)

func TestValid(t *testing.T) {
	if !Valid("dpapi:YmxvYg==") {
		t.Fatal("un valor dpapi en base64 debería aceptarse")
	}
	for _, bad := range []string{"", "temporal", "dpapi:", "dpapi:!!!", "YmxvYg=="} {
		if Valid(bad) {
			t.Fatalf("no debería aceptarse %q", bad)
		}
	}
}

func TestProtectRoundTrip(t *testing.T) {
	const password = "clave-con-eñe-y-símbolos-€"
	encoded, err := Protect(password)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if !Valid(encoded) {
		t.Fatalf("formato inesperado: %s", encoded)
	}
	if strings.Contains(encoded, password) {
		t.Fatal("el valor guardado contiene la contraseña en claro")
	}
	got, err := Open(encoded)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got != password {
		t.Fatal("la contraseña descifrada no coincide")
	}
	if _, err := Open("dpapi:YmxvYg=="); err == nil {
		t.Fatal("un blob falso debería fallar al descifrar")
	}
}
