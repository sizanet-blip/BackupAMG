package status

import "time"

// Estados de una copia adicional en el panel.
const (
	ReplicaOK       = "ok"       // al día
	ReplicaPending  = "pending"  // le falta la última copia, pero todo va bien
	ReplicaOffline  = "offline"  // el disco o la carpeta no están conectados
	ReplicaFailed   = "failed"   // el último intento falló
	ReplicaDisabled = "disabled" // pausada por el usuario
)

// Replica es una copia adicional tal como la ve Inicio. No lleva rutas: la ven todos los usuarios.
type Replica struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	State         string `json:"state"`
	Stale         bool   `json:"stale,omitempty"`
	DaysBehind    int    `json:"days_behind,omitempty"`
	LastSuccessAt string `json:"last_success_at,omitempty"`
	Message       string `json:"message,omitempty"`
	Running       bool   `json:"running,omitempty"`
}

// ReplicaFacts son los datos de una copia adicional con los que se calcula su estado.
type ReplicaFacts struct {
	ID, Name      string
	Enabled       bool
	Pending       bool
	LastStatus    string // success, failed, offline, interrupted o vacío
	LastError     string
	LastSuccessAt string
	AddedAt       string // desde cuándo existe, si todavía no se ha replicado nunca
	Running       bool
	Now           time.Time
	StaleAfter    time.Duration
}

// DescribeReplica calcula el estado y si va retrasada.
// Va retrasada si le falta la última copia y lleva más de StaleAfter sin ponerse al día
// (contando desde su última réplica correcta o, si no hay, desde que se añadió).
func DescribeReplica(f ReplicaFacts) Replica {
	out := Replica{ID: f.ID, Name: f.Name, LastSuccessAt: f.LastSuccessAt, Running: f.Running}
	switch {
	case !f.Enabled:
		out.State = ReplicaDisabled
		return out
	case !f.Pending:
		out.State = ReplicaOK
		return out
	case f.LastStatus == "offline":
		out.State = ReplicaOffline
	case f.LastStatus == "failed":
		out.State = ReplicaFailed
		out.Message = f.LastError
	default:
		out.State = ReplicaPending
	}
	ref := f.LastSuccessAt
	if ref == "" {
		ref = f.AddedAt
	}
	if since, err := time.Parse(time.RFC3339, ref); err == nil && f.StaleAfter > 0 && f.Now.Sub(since) > f.StaleAfter {
		out.Stale = true
		out.DaysBehind = int(f.Now.Sub(since) / (24 * time.Hour))
	}
	return out
}
