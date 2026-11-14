package main

import "testing"

func TestMenuHasTheEntriesOfTheDesignSpec(t *testing.T) {
	entries := menuEntries(true, false)
	want := []string{"Abrir panel", "Copiar ahora", "Ver historial", "Restaurar archivos", "Ocultar el icono"}
	if len(entries) != len(want) {
		t.Fatalf("entradas: %d", len(entries))
	}
	for i, label := range want {
		if entries[i].label != label {
			t.Fatalf("entrada %d: %q, se esperaba %q", i, entries[i].label, label)
		}
		if entries[i].bold != (i == 0) {
			t.Fatalf("solo «Abrir panel» va en negrita: %q", entries[i].label)
		}
	}
	if entries[4].hint != "Las copias siguen funcionando" || entries[4].action != menuHide {
		t.Fatalf("ocultar el icono: %+v", entries[4])
	}
}

func TestMenuAvailabilityFollowsTheService(t *testing.T) {
	stopped := menuEntries(false, false)
	for _, entry := range stopped[:4] {
		if entry.enabled {
			t.Fatalf("%q no debe poder usarse con el servicio parado", entry.label)
		}
	}
	if !stopped[4].enabled {
		t.Fatal("ocultar el icono debe poder usarse siempre")
	}
	busy := menuEntries(true, true)
	if busy[1].enabled {
		t.Fatal("no se puede copiar ahora mientras ya hay una tarea")
	}
	if !busy[0].enabled || !busy[2].enabled || !busy[3].enabled {
		t.Fatal("el resto de opciones siguen disponibles durante una copia")
	}
}
