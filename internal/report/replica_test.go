package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Los informes de copia no ganan campos de réplica: los lectores de antes no ven nada nuevo.
func TestBackupReportHasNoReplicaFields(t *testing.T) {
	now := time.Now()
	data, err := json.Marshal(Build(Input{Started: now, Finished: now, Status: "success"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"replica_id", "replica_name", "snapshots_copied"} {
		if strings.Contains(string(data), key) {
			t.Fatalf("%s no debe aparecer en un informe de copia: %s", key, data)
		}
	}
}

func TestReplicaReportKeepsCommonFieldsAndSchema(t *testing.T) {
	now := time.Now()
	rep := Build(Input{
		Type: TypeReplica, Started: now, Finished: now.Add(time.Minute), Status: "success",
		Repository: `F:\CopiasExtra`, ReplicaID: "usb", ReplicaName: "Disco externo", SnapshotsCopied: 2,
		Message: "Copia adicional actualizada",
	})
	data, _ := json.Marshal(rep)
	var back map[string]any
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back["schema_version"].(float64) != 1 || back["type"] != "replica" || back["replica_id"] != "usb" || back["snapshots_copied"].(float64) != 2 {
		t.Fatalf("%v", back)
	}
	// Un lector de antes (con la estructura sin los campos nuevos) sigue leyéndolo.
	var old struct {
		SchemaVersion int      `json:"schema_version"`
		Type          string   `json:"type"`
		Status        string   `json:"status"`
		Repository    string   `json:"repository"`
		Errors        []string `json:"errors"`
	}
	if err := json.Unmarshal(data, &old); err != nil || old.Repository != `F:\CopiasExtra` || old.Errors == nil {
		t.Fatalf("%+v %v", old, err)
	}
}
