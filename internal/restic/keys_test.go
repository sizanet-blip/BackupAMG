package restic

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKeysAddListRemoveAndTheOtherPasswordMessage(t *testing.T) {
	exe := replicaExe(t)
	root := t.TempDir()
	primary := filepath.Join(root, "principal")
	replica := filepath.Join(root, "adicional")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if _, err := Init(ctx, InitRequest{Executable: exe, Repository: primary, Password: "vieja"}); err != nil {
		t.Fatal(err)
	}
	if _, err := InitReplica(ctx, ReplicaInitRequest{Executable: exe, Repository: replica, Source: primary, Password: "vieja"}); err != nil {
		t.Fatal(err)
	}

	oldID, err := CurrentKeyID(ctx, exe, primary, "vieja")
	if err != nil || len(oldID) != 64 {
		t.Fatalf("CurrentKeyID: %q %v", oldID, err)
	}
	if _, err := CurrentKeyID(ctx, exe, primary, "equivocada"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("con otra contraseña debía ser ErrWrongPassword: %v", err)
	}
	newID, err := KeyAdd(ctx, exe, primary, "vieja", "nueva")
	if err != nil || len(newID) != 64 || newID == oldID {
		t.Fatalf("KeyAdd: %q %v", newID, err)
	}
	keys, err := KeyList(ctx, exe, primary, "vieja")
	if err != nil || len(keys) != 2 {
		t.Fatalf("KeyList: %v %v", keys, err)
	}
	// No se puede quitar la clave con la que se está abriendo.
	if err := KeyRemove(ctx, exe, primary, "nueva", newID); err == nil {
		t.Fatal("no debe dejar quitar la clave actual")
	}
	if err := KeyRemove(ctx, exe, primary, "nueva", "no-es-un-id"); err == nil {
		t.Fatal("un identificador no válido debía rechazarse")
	}
	if err := KeyRemove(ctx, exe, primary, "nueva", oldID); err != nil {
		t.Fatal(err)
	}
	if ok, _, _ := PasswordOK(ctx, exe, primary, "vieja"); ok {
		t.Fatal("la vieja ya no debía abrir")
	}

	// La principal ya usa «nueva» pero la adicional sigue con «vieja»: el error lo dice claro.
	_, err = Copy(ctx, CopyRequest{Executable: exe, Source: primary, Destination: replica, Password: "nueva"})
	if !errors.Is(err, ErrReplicaOtherPassword) {
		t.Fatalf("debía ser ErrReplicaOtherPassword: %v", err)
	}
	if strings.Contains(err.Error(), "nueva") || strings.Contains(err.Error(), "vieja") {
		t.Fatalf("el mensaje contiene una contraseña: %v", err)
	}
	if PlainFailure(err.Error()) != err.Error() {
		t.Fatalf("el mensaje no debe pasar por el filtro técnico: %q", PlainFailure(err.Error()))
	}
}
